// Package rabbitmq provides a RabbitMQ backend for pkg/mq using amqp091.
//
// Architecture summary:
//   - A resource.Provider[*amqp091.Connection] owns the AMQP connection and
//     rebuilds it on config change or health check failure.
//   - Publisher wraps the provider with an SRE circuit-breaker; each Publish
//     opens a fresh channel, publishes, and closes the channel.
//   - Consumer.Subscribe delegates to mq.RunSupervised; its ConnectFn opens a
//     channel, declares the queue, and starts an AMQP consumer returning a
//     <-chan mq.Delivery adapter.
package rabbitmq

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/go-kratos/aegis/circuitbreaker"
	"github.com/go-kratos/aegis/circuitbreaker/sre"

	"github.com/z-mate/kratos-base/pkg/errs"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

// Config holds the parameters needed to connect to a RabbitMQ broker.
//
// Routing model: this adapter uses the AMQP default exchange only. The queue
// name IS the topic — Consumer declares/consumes the queue named after the
// topic, and Publisher routes with routing key == m.Topic. Custom exchanges
// are intentionally out of scope until a real need shows up (YAGNI); when
// they do, add the field back together with real declare/bind/publish wiring.
type Config struct {
	// URL is the AMQP connection URL, e.g. "amqp://guest:guest@localhost:5672/".
	URL string
	// DialTimeout caps the TCP dial phase. Zero uses a 5-second default.
	DialTimeout time.Duration
	// PrefetchCount bounds the number of unacknowledged deliveries the broker
	// pushes to a single consumer at once (manual-ack QoS, R10F2). Zero uses a
	// sensible default aligned with the consumer's out-channel buffer
	// (defaultPrefetch). Without it the broker would shove the whole ready
	// backlog onto one consumer, leaving the in-flight set unbounded (OOM risk).
	PrefetchCount int
	// MaxDeliveryAttempts bounds how many times a single message may be
	// delivered before the BROKER routes it to the dead-letter queue instead of
	// redelivering it (R11F9 poison-message guard). Zero uses
	// defaultMaxDeliveryAttempts. Counting from 1 = first delivery, so a value
	// of 5 means up to 4 redeliveries, then dead-letter.
	//
	// This is enforced broker-side via a quorum main queue declared with
	// x-delivery-limit = MaxDeliveryAttempts-1 (see topicQueueArgs): the quorum
	// queue type maintains an authoritative x-delivery-count per message and
	// dead-letters automatically once the count exceeds the limit. The consumer
	// just Nack(requeue=true)s a failed delivery; it does NOT count attempts in
	// application code. (Classic queues cannot do this: a Nack(requeue=true)
	// reinserts the message without advancing any broker-maintained counter, so
	// a poison message would hot-loop forever — the R10 bug this replaces.)
	MaxDeliveryAttempts int
}

func (c Config) dialTimeout() time.Duration {
	if c.DialTimeout <= 0 {
		return 5 * time.Second
	}
	return c.DialTimeout
}

// defaultPrefetch is the manual-ack QoS prefetch used when Config.PrefetchCount
// is unset. It matches the adapter's out-channel buffer (see makeConnectFn) so
// the broker keeps roughly one buffer-worth of in-flight work staged, no more.
const defaultPrefetch = 64

func (c Config) prefetchCount() int {
	if c.PrefetchCount <= 0 {
		return defaultPrefetch
	}
	return c.PrefetchCount
}

// defaultMaxDeliveryAttempts is the poison-message redelivery ceiling used when
// Config.MaxDeliveryAttempts is unset. After this many delivery attempts the
// broker dead-letters the message (quorum x-delivery-limit) instead of
// redelivering it.
const defaultMaxDeliveryAttempts = 5

func (c Config) maxDeliveryAttempts() int {
	if c.MaxDeliveryAttempts <= 0 {
		return defaultMaxDeliveryAttempts
	}
	return c.MaxDeliveryAttempts
}

// Fingerprint returns a stable hex string that covers the connection-relevant
// fields. Equal configs produce equal fingerprints; changing URL or
// DialTimeout changes the fingerprint.
func (c Config) Fingerprint() string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s", c.URL, c.dialTimeout())
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ─────────────────────────────────────────────────────────────────────────────
// StaticSource — convenience wrapper so tests can feed a Config directly.
// ─────────────────────────────────────────────────────────────────────────────

// StaticSource wraps a single Config as a resource.Source. The version is
// always 1; it never changes.
type StaticSource Config

func (s StaticSource) Current() resource.Snapshot {
	return resource.Snapshot{Version: 1, Value: Config(s)}
}

// liveConn is the minimal read view getLiveConn needs to decide whether a
// cached connection is a corpse. *amqp.Connection satisfies it; a fake in
// tests satisfies it too, which lets the dead-connection rebuild branch be
// unit-tested without a live broker.
type liveConn interface {
	IsClosed() bool
}

// connSource is the injectable connection seam getLiveConn drives: a Get that
// returns the current (possibly dead) connection and a Close that invalidates
// the cached handle so the next Get rebuilds. The production implementation
// (providerConnSource) wraps a *resource.Provider[*amqp.Connection]; tests
// inject a fake whose first Get returns a closed connection so the
// IsClosed→Close→retry-Get-once self-heal branch can be hard-asserted.
type connSource interface {
	// Get returns the current connection (nil iff err != nil is not required;
	// getLiveConn handles a nil/closed conn explicitly).
	Get(ctx context.Context) (liveConn, error)
	// Close invalidates the cached connection so the next Get rebuilds.
	Close() error
}

// providerConnSource adapts a *resource.Provider[*amqp.Connection] to
// connSource. Get returns the provider's connection widened to liveConn; a
// typed-nil *amqp.Connection is normalized to an untyped nil so getLiveConn's
// `conn == nil` guard behaves correctly.
type providerConnSource struct {
	p *resource.Provider[*amqp.Connection]
}

func (s providerConnSource) Get(ctx context.Context) (liveConn, error) {
	conn, err := s.p.Get(ctx)
	if conn == nil {
		// Avoid returning a non-nil interface wrapping a typed-nil pointer,
		// which would make `conn == nil` in getLiveConn false unexpectedly.
		return nil, err
	}
	return conn, err
}

func (s providerConnSource) Close() error { return s.p.Close() }

// getLiveConn returns a live AMQP connection from the seam.
//
// Crucial self-heal detail: when the broker dies, the provider still caches
// the dead *amqp.Connection — config version and fingerprint are unchanged,
// so a plain Get keeps returning the corpse forever. Detect IsClosed, force
// the provider to drop it (Close marks not-ready), and retry Get ONCE so a
// rebuild is attempted immediately; if the broker is still down that Build
// fails and the caller's retry/backoff path takes over.
//
// Returns liveConn so the self-heal logic is broker-free testable; production
// callers type-assert the result back to *amqp.Connection to open a channel.
func getLiveConn(ctx context.Context, s connSource) (liveConn, error) {
	conn, err := s.Get(ctx)
	if err == nil && (conn == nil || conn.IsClosed()) {
		_ = s.Close() // invalidate the dead cached handle
		conn, err = s.Get(ctx)
	}
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.IsClosed() {
		return nil, fmt.Errorf("amqp connection closed")
	}
	return conn, nil
}

// amqpConn type-asserts a liveConn (as returned by getLiveConn over a
// providerConnSource) back to the concrete *amqp.Connection the publish and
// consume paths need to open a channel. In production this assertion always
// holds; a failure means a fake leaked into a non-test path.
func amqpConn(c liveConn) (*amqp.Connection, error) {
	conn, ok := c.(*amqp.Connection)
	if !ok {
		return nil, fmt.Errorf("rabbitmq: expected *amqp.Connection, got %T", c)
	}
	return conn, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Connection provider
// ─────────────────────────────────────────────────────────────────────────────

func cfgFrom(v any) Config {
	if c, ok := v.(Config); ok {
		return c
	}
	return Config{}
}

// newConnProvider builds a resource.Provider[*amqp.Connection] from src.
func newConnProvider(src resource.Source) *resource.Provider[*amqp.Connection] {
	ad := resource.Adapter[*amqp.Connection]{
		Build: func(ctx context.Context, v any) (*amqp.Connection, error) {
			cfg := cfgFrom(v)
			timeout := cfg.dialTimeout()

			amqpCfg := amqp.Config{
				Dial: func(network, addr string) (net.Conn, error) {
					d := net.Dialer{Timeout: timeout}
					return d.DialContext(ctx, network, addr)
				},
			}
			conn, err := amqp.DialConfig(cfg.URL, amqpCfg)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
		Close: func(conn *amqp.Connection) error {
			if conn != nil && !conn.IsClosed() {
				return conn.Close()
			}
			return nil
		},
		Fingerprint: func(v any) string {
			return cfgFrom(v).Fingerprint()
		},
		Health: func(_ context.Context, conn *amqp.Connection) error {
			if conn == nil || conn.IsClosed() {
				return errs.DBUnavailable(fmt.Errorf("amqp connection closed"))
			}
			return nil
		},
	}
	return resource.New[*amqp.Connection](src, ad)
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue topology: quorum main queue + dead-letter queue (DLQ)
//
// Poison-message containment (R11F9, replacing the broken R10 approach): a
// handler that always fails would, with a plain Nack(requeue=true) and no
// ceiling, hot-loop the same message forever (zero-backoff infinite poison
// loop). We bound it BROKER-SIDE by declaring the main queue as a quorum queue
// (x-queue-type=quorum) with x-delivery-limit = MaxDeliveryAttempts-1 and a
// dead-letter exchange pointing at a companion DLQ. The quorum queue type keeps
// an authoritative per-message x-delivery-count; once a message's redelivery
// count exceeds x-delivery-limit the broker automatically dead-letters it to
// the DLQ. The consumer therefore just Nack(requeue=true)s a failed delivery —
// no application-side attempt counting.
//
// Why not the R10 approach (classic queue + count Redelivered/x-death):
// on a classic queue a Nack(requeue=true) reinserts the message WITHOUT bumping
// x-death and only sets the Redelivered BOOLEAN (it cannot advance past 2), so
// an attempt counter built on it pins below the ceiling and the requeue stays
// true forever — the poison message never reaches the DLQ. x-death is only bumped
// by an actual dead-letter cycle, which never starts. Quorum's x-delivery-limit
// is the broker-native fix; rabbitmq:3-management-alpine supports it.
//
// Idempotent-declare consistency: RabbitMQ rejects a QueueDeclare that names an
// existing queue with DIFFERENT arguments (PRECONDITION_FAILED). The publisher
// and the consumer both declare the main queue, so they MUST pass byte-identical
// args (queue type, delivery limit, DLX) — hence the single shared
// topicQueueArgs/declareTopicQueue helpers used by both, parameterised by the
// SAME resolved maxAttempts. The DLQ name is derived purely from the topic
// (deterministic, no config), so the two sides never disagree on topology.
// ─────────────────────────────────────────────────────────────────────────────

// dlqSuffix is appended to a topic to name its dead-letter queue. Routing uses
// the default exchange, so the DLQ name doubles as its routing key.
const dlqSuffix = ".dlq"

// dlqName returns the dead-letter queue name for topic.
func dlqName(topic string) string { return topic + dlqSuffix }

// topicQueueArgs returns the x-args the main queue is declared with:
//
//   - x-queue-type=quorum: makes the broker maintain an authoritative
//     per-message x-delivery-count and honour x-delivery-limit (a classic queue
//     ignores x-delivery-limit entirely — see topology note above).
//   - x-delivery-limit = maxAttempts-1: the broker dead-letters a message once
//     its delivery count EXCEEDS this limit. maxAttempts is 1-based (the first
//     delivery is attempt 1), so a limit of maxAttempts-1 yields exactly
//     maxAttempts total deliveries before dead-lettering.
//   - dead-letter exchange/routing-key: a Nack-driven dead-letter (or a
//     limit-exceeded dead-letter) lands in the companion DLQ rather than being
//     dropped. Default exchange "" + routing key == the DLQ name.
//
// MUST be identical on the publisher and consumer declare paths (see topology
// note) — that is why it is a single shared function taking the SAME resolved
// maxAttempts on both sides.
func topicQueueArgs(topic string, maxAttempts int) amqp.Table {
	return amqp.Table{
		"x-queue-type":              "quorum",
		"x-delivery-limit":          maxAttempts - 1, // 1-based attempts → limit is one less
		"x-dead-letter-exchange":    "",              // default exchange: route by queue name
		"x-dead-letter-routing-key": dlqName(topic),
	}
}

// declareTopicQueue declares the durable quorum main queue for topic with the
// delivery-limit + dead-letter args. Used by BOTH the publisher
// (ensure-exists-before-publish) and the consumer (ensure-exists-before-consume)
// with the SAME resolved maxAttempts so the declares are byte-for-byte idempotent
// (a mismatch would be rejected with PRECONDITION_FAILED).
func declareTopicQueue(ch *amqp.Channel, topic string, maxAttempts int) error {
	_, err := ch.QueueDeclare(
		topic, // name == topic == routing key
		true,  // durable (quorum queues are always durable)
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		topicQueueArgs(topic, maxAttempts),
	)
	return err
}

// declareDLQ declares the durable dead-letter queue for topic. Only the consumer
// needs to call this (it owns consumption); declaring it guarantees the queue
// exists before any message is dead-lettered, otherwise the broker would route
// the dead-letter to a non-existent queue and silently drop the poison message.
func declareDLQ(ch *amqp.Channel, topic string) error {
	_, err := ch.QueueDeclare(
		dlqName(topic),
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // plain durable queue: the DLQ is the terminus — no delivery limit,
		//        no onward dead-lettering, so no quorum/DLX args needed here.
	)
	return err
}

// ─────────────────────────────────────────────────────────────────────────────
// Publisher
// ─────────────────────────────────────────────────────────────────────────────

// Publisher implements mq.Publisher for RabbitMQ.
type Publisher struct {
	provider *resource.Provider[*amqp.Connection]
	connSrc  connSource
	breaker  circuitbreaker.CircuitBreaker
	cfg      Config // resolved once: drives the main-queue delivery limit on declare
}

// NewPublisher creates a Publisher backed by src with a production SRE breaker.
func NewPublisher(src resource.Source) *Publisher {
	return NewPublisherWithBreaker(src, sre.NewBreaker())
}

// NewPublisherWithBreaker is like NewPublisher but injects the circuit breaker.
// Production code uses NewPublisher (sre.NewBreaker); tests inject a
// deterministic stub to assert the open-circuit fast-fail path (R2F3/R2F10).
func NewPublisherWithBreaker(src resource.Source, breaker circuitbreaker.CircuitBreaker) *Publisher {
	p := newConnProvider(src)
	return &Publisher{
		provider: p,
		connSrc:  providerConnSource{p: p},
		breaker:  breaker,
		// Resolve the config once so the publisher's main-queue declare uses the
		// SAME x-delivery-limit as the consumer's (else the second declare is
		// rejected PRECONDITION_FAILED).
		cfg: cfgFrom(src.Current().Value),
	}
}

// buildPublishTable builds the outgoing AMQP header table: it copies the
// business headers and injects the current span's W3C trace context so the async
// hop carries the trace to the consumer (R9F6). mq.InjectTrace uses a
// self-contained propagation.TraceContext{} (not the global propagator) and
// returns a copy, so the caller's m.Headers map is never mutated. Pure (no
// broker), hence unit-testable: see TestBuildPublishTable_InjectsTrace.
func buildPublishTable(ctx context.Context, m mq.Message) amqp.Table {
	table := amqp.Table{}
	for k, v := range mq.InjectTrace(ctx, m.Headers) {
		table[k] = v
	}
	return table
}

// failPublish reports a publish-path failure as DBUnavailable, but feeds the
// breaker only when err reflects a BACKEND fault — not a caller hang-up (R10F4).
//
// A client that explicitly cancels its ctx mid-publish (HTTP client disconnect,
// upstream deadline propagation, etc.) is NOT evidence the broker is sick;
// counting it as a failure (MarkFailed) would let a burst of disconnects trip
// the breaker against a perfectly healthy broker. So when errs.IsCallerCanceled
// holds we skip MarkFailed (we record no verdict — neither failure nor success)
// and still surface the wrapped error to the caller.
//
// Note the asymmetry with timeouts: context.DeadlineExceeded is deliberately
// NOT exempt (IsCallerCanceled returns false for it) — a blown deadline may well
// be a slow/sick broker, so it must still MarkFailed.
func (p *Publisher) failPublish(ctx context.Context, err error) error {
	if !errs.IsCallerCanceled(ctx, err) {
		p.breaker.MarkFailed()
	}
	return errs.DBUnavailable(err)
}

// Publish sends m to the broker.
//
// Circuit-breaker gating: if the breaker is open, returns DBUnavailable
// immediately without attempting a dial.
func (p *Publisher) Publish(ctx context.Context, m mq.Message) error {
	// With the default exchange the routing key must equal the queue name,
	// and the queue name IS the topic (see Config doc). An empty topic would
	// be silently dropped by the broker — fail loud instead.
	if m.Topic == "" {
		return errs.InvalidArgument("mq: message topic must not be empty")
	}
	if err := p.breaker.Allow(); err != nil {
		return errs.DBUnavailable(err)
	}

	lc, err := getLiveConn(ctx, p.connSrc)
	if err != nil {
		return p.failPublish(ctx, err)
	}
	conn, err := amqpConn(lc)
	if err != nil {
		return p.failPublish(ctx, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return p.failPublish(ctx, err)
	}
	defer ch.Close() //nolint:errcheck

	// Ensure the destination queue exists BEFORE publishing. With the default
	// exchange an unroutable message (queue not yet declared — e.g. the
	// consumer hasn't reconnected after a broker outage) is silently dropped
	// by the broker. Declaring here (idempotent, SAME args as the consumer via
	// the shared declareTopicQueue helper + the same resolved maxAttempts)
	// guarantees messages published during a consumer-outage window are parked
	// durably until the consumer re-subscribes.
	if err := declareTopicQueue(ch, m.Topic, p.cfg.maxDeliveryAttempts()); err != nil {
		return p.failPublish(ctx, err)
	}

	// Build the outgoing AMQP header table (business headers + injected trace).
	// buildPublishTable is a pure helper so the R9F6 trace-inject wiring is
	// unit-testable without a broker — see TestBuildPublishTable_InjectsTrace.
	table := buildPublishTable(ctx, m)

	err = ch.PublishWithContext(ctx,
		"",      // default exchange: routing key must equal the queue name
		m.Topic, // routing key == queue name == topic（消费侧按 topic 声明队列）
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			Headers:      table,
			ContentType:  "application/octet-stream",
			DeliveryMode: amqp.Persistent, // survive broker restart while parked in the durable queue
			MessageId:    m.Key,           // business id rides along; never used for routing
			Body:         m.Body,
		},
	)
	if err != nil {
		return p.failPublish(ctx, err)
	}

	p.breaker.MarkSuccess()
	return nil
}

// Close tears down the underlying connection provider.
func (p *Publisher) Close() error {
	return p.provider.Close()
}

// Healthy drives a connection refresh and probes the underlying AMQP connection.
// It satisfies resource.Check and can be registered directly into a
// resource.Registry:
//
//	reg.Register("mq", pub.Healthy)
func (p *Publisher) Healthy(ctx context.Context) error {
	return p.provider.Healthy(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Consumer
// ─────────────────────────────────────────────────────────────────────────────

// Consumer implements mq.Consumer for RabbitMQ using RunSupervised.
type Consumer struct {
	provider *resource.Provider[*amqp.Connection]
	connSrc  connSource
	cfg      Config // resolved once: drives prefetch QoS and the poison ceiling
}

// NewConsumer creates a Consumer backed by src.
func NewConsumer(src resource.Source) *Consumer {
	p := newConnProvider(src)
	return &Consumer{
		provider: p,
		connSrc:  providerConnSource{p: p},
		cfg:      cfgFrom(src.Current().Value),
	}
}

// Subscribe starts the supervised consumer loop. It blocks until ctx is
// cancelled. The implementation reconnects automatically on channel or
// connection failures.
func (c *Consumer) Subscribe(ctx context.Context, topic string, h mq.Handler) error {
	connectFn := c.makeConnectFn(topic)
	return mq.RunSupervised(ctx, topic, h, connectFn, mq.DefaultBackoff)
}

// makeConnectFn returns a ConnectFn that opens a fresh AMQP channel,
// declares the queue, and starts a consumer. The returned chan is closed
// when the AMQP channel is cancelled or closed.
func (c *Consumer) makeConnectFn(topic string) mq.ConnectFn {
	return func(ctx context.Context, _ string) (<-chan mq.Delivery, func(), error) {
		lc, err := getLiveConn(ctx, c.connSrc)
		if err != nil {
			return nil, nil, errs.DBUnavailable(err)
		}
		conn, err := amqpConn(lc)
		if err != nil {
			return nil, nil, errs.DBUnavailable(err)
		}

		ch, err := conn.Channel()
		if err != nil {
			return nil, nil, errs.DBUnavailable(err)
		}

		// Declare the dead-letter queue FIRST, before the main queue references
		// it: if the DLQ did not exist when a poison message is dead-lettered,
		// the broker would route the dead-letter to a non-existent queue and
		// silently drop the message — defeating the whole poison-containment.
		if err := declareDLQ(ch, topic); err != nil {
			ch.Close() //nolint:errcheck
			return nil, nil, errs.DBUnavailable(err)
		}

		// Declare the quorum main queue (durable; survives broker restarts) WITH
		// the x-delivery-limit + dead-letter args, via the shared declareTopicQueue
		// helper so the args are byte-for-byte identical to the publisher's declare
		// — same resolved maxAttempts on both sides (else RabbitMQ would reject the
		// second declare with PRECONDITION_FAILED). Default-exchange model: queue
		// name == topic; the publisher routes with routing key == topic, so no
		// exchange declare/bind is needed. The broker (quorum queue) enforces the
		// delivery limit and dead-letters poison messages automatically.
		if err := declareTopicQueue(ch, topic, c.cfg.maxDeliveryAttempts()); err != nil {
			ch.Close() //nolint:errcheck
			return nil, nil, errs.DBUnavailable(err)
		}

		// QoS/prefetch BEFORE Consume (R10F2): without it the broker pushes the
		// entire ready backlog to this single manual-ack consumer at once,
		// making the in-flight (unacknowledged) set unbounded — a memory risk
		// under backlog. Bound it to prefetchCount unacked deliveries.
		if err := ch.Qos(c.cfg.prefetchCount(), 0, false); err != nil {
			ch.Close() //nolint:errcheck
			return nil, nil, errs.DBUnavailable(err)
		}

		amqpDeliveries, err := ch.Consume(
			topic, // queue
			"",    // consumer tag (auto)
			false, // auto-ack (manual ack for reliability)
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			ch.Close() //nolint:errcheck
			return nil, nil, errs.DBUnavailable(err)
		}

		out := make(chan mq.Delivery, defaultPrefetch)
		go adaptDeliveries(ctx, amqpDeliveries, out)

		cleanup := func() {
			ch.Close() //nolint:errcheck
		}
		return out, cleanup, nil
	}
}

// adaptDeliveries converts <-chan amqp.Delivery to <-chan mq.Delivery.
// When the amqp channel closes, out is also closed.
//
// Poison containment (R11F9): the Nack closure always Nacks with requeue=true.
// On the QUORUM main queue the broker maintains the authoritative
// x-delivery-count and, once it exceeds x-delivery-limit (= maxAttempts-1, set
// at declare time — see topicQueueArgs), dead-letters the message to the DLQ
// automatically. So the consumer does NOT count attempts; a failed handler just
// requeues and the broker breaks the loop. (Contrast the broken R10 design,
// which tried to count Redelivered/x-death in app code and Nack(requeue=false)
// at a ceiling — on a classic queue that counter never advanced, so the message
// hot-looped forever.)
//
// Leak safety: the send into out is made cancellable on ctx. If the consumer
// loop has stopped reading (ctx cancelled) while out's buffer is full, a plain
// `out <- dd` would block forever and leak this goroutine. Selecting on
// ctx.Done() lets the adapter exit instead. close(out) still runs via defer.
func adaptDeliveries(ctx context.Context, in <-chan amqp.Delivery, out chan<- mq.Delivery) {
	defer close(out)
	for d := range in {
		headers := make(map[string]string, len(d.Headers))
		for k, v := range d.Headers {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
		dd := d // capture for closures
		md := mq.Delivery{
			Msg: mq.Message{
				Topic:   d.RoutingKey,
				Key:     d.MessageId, // mirrors Publish: business id rides in MessageId
				Body:    d.Body,
				Headers: headers,
			},
			Ack: func() error { return dd.Ack(false) },
			// Always requeue: the quorum main queue's x-delivery-limit (set at
			// declare time) makes the broker dead-letter the message once its
			// redelivery count exceeds the limit. No app-side attempt counting.
			Nack: func() error { return dd.Nack(false, true) },
		}
		select {
		case out <- md:
		case <-ctx.Done():
			return
		}
	}
}

// Close tears down the underlying connection provider.
func (c *Consumer) Close() error {
	return c.provider.Close()
}
