package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/z-mate/kratos-base/pkg/mq"
)

// AdaptDeliveries exposes the unexported adaptDeliveries to the external
// rabbitmq_test package so its pure field-mapping / leak behavior can be
// hard-asserted without spinning up a broker. Poison-message dead-lettering is
// now broker-side (quorum x-delivery-limit, R11F9), so the adapter no longer
// takes a ceiling — the Nack closure always requeues.
func AdaptDeliveries(ctx context.Context, in <-chan amqp.Delivery, out chan<- mq.Delivery) {
	adaptDeliveries(ctx, in, out)
}

// DLQName exposes the unexported dlqName so tests can assert the dead-letter
// routing key the main queue is declared with.
func DLQName(topic string) string { return dlqName(topic) }

// ResolvedPrefetch exposes the resolved Config.prefetchCount (R10F2 QoS
// default). The QoS wiring itself needs a live broker, but the chosen default is
// load-bearing behaviour, so the resolution is unit-tested via this seam.
func ResolvedPrefetch(c Config) int { return c.prefetchCount() }

// ResolvedMaxAttempts exposes the resolved Config.maxDeliveryAttempts (the
// poison-ceiling default, R10F1/R10F5).
func ResolvedMaxAttempts(c Config) int { return c.maxDeliveryAttempts() }

// TopicQueueArgs exposes the unexported topicQueueArgs so tests can pin the
// quorum-type + x-delivery-limit + dead-letter args the publisher and consumer
// MUST declare identically (these args are what actually drive broker-side
// poison-message dead-lettering, R11F9).
func TopicQueueArgs(topic string, maxAttempts int) amqp.Table {
	return topicQueueArgs(topic, maxAttempts)
}

// LiveConn is the exported alias of the internal connection read view a fake
// connection must satisfy (just IsClosed). Used by getLiveConn self-heal tests.
type LiveConn = liveConn

// ConnSource is the exported alias of the internal injectable connection seam
// (Get + Close) that getLiveConn drives. Tests inject a fake whose first Get
// returns a closed connection to exercise the IsClosed→Close→retry-Get-once
// rebuild branch (R2F4).
type ConnSource = connSource

// GetLiveConn drives the unexported getLiveConn self-heal logic with the
// supplied connection seam so the dead-connection rebuild branch can be
// hard-asserted without a live broker.
func GetLiveConn(ctx context.Context, s ConnSource) (LiveConn, error) {
	return getLiveConn(ctx, s)
}

// BuildPublishTable drives the unexported buildPublishTable helper (the exact
// header table the publish path sends) so the R9F6 publisher-side trace injection
// can be asserted without a live broker. Returns the AMQP table's string values
// as a plain map for easy assertion.
func BuildPublishTable(ctx context.Context, m mq.Message) map[string]string {
	t := buildPublishTable(ctx, m)
	out := make(map[string]string, len(t))
	for k, v := range t {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
