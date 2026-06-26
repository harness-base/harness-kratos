// Package data – MQ adapter for the demo service.
// Wires mq.Publisher and mq.Consumer into resource.Provider-backed backends
// using the configured broker kind (rabbitmq / rocketmq). When kind is ""
// (disabled) all fields remain nil and MQHealthy returns nil so the readiness
// gate is silent.
package data

import (
	"context"

	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// runtimeRabbitSource maps a full Runtime snapshot to rabbitmq.Config.
// It is a resource.Source so the rabbitmq provider can react to hot-reload.
type runtimeRabbitSource struct {
	src        resource.Source
	selectMQFn func(any) (string, rabbitmq.Config, rocketmq.Config, string, error)
}

func (s *runtimeRabbitSource) Current() resource.Snapshot {
	snap := s.src.Current()
	_, rabbitCfg, _, _, err := s.selectMQFn(snap.Value)
	if err != nil {
		return resource.Snapshot{Version: snap.Version, Value: rabbitmq.Config{}}
	}
	return resource.Snapshot{Version: snap.Version, Value: rabbitCfg}
}

// runtimeRocketSource maps a full Runtime snapshot to rocketmq.Config.
type runtimeRocketSource struct {
	src        resource.Source
	selectMQFn func(any) (string, rabbitmq.Config, rocketmq.Config, string, error)
}

func (s *runtimeRocketSource) Current() resource.Snapshot {
	snap := s.src.Current()
	_, _, rocketCfg, _, err := s.selectMQFn(snap.Value)
	if err != nil {
		return resource.Snapshot{Version: snap.Version, Value: rocketmq.Config{}}
	}
	return resource.Snapshot{Version: snap.Version, Value: rocketCfg}
}

// initMQ constructs the MQ publisher/consumer and stores them on d.
// selectMQFn must return (kind, rabbitmqCfg, rocketmqCfg, topic, error).
// When kind == "" or selectMQFn returns an error, MQ is left disabled.
func initMQ(
	d *Data,
	src resource.Source,
	selectMQFn func(any) (string, rabbitmq.Config, rocketmq.Config, string, error),
) {
	snap := src.Current()
	kind, _, _, topic, err := selectMQFn(snap.Value)
	if err != nil || kind == "" {
		return
	}

	switch kind {
	case "rabbitmq":
		rabbitSrc := &runtimeRabbitSource{src: src, selectMQFn: selectMQFn}
		pub := rabbitmq.NewPublisher(rabbitSrc)
		con := rabbitmq.NewConsumer(rabbitSrc)
		d.mqPublisher = pub
		d.mqConsumer = con
		d.mqHealthy = pub.Healthy
	case "rocketmq":
		rocketSrc := &runtimeRocketSource{src: src, selectMQFn: selectMQFn}
		pub := rocketmq.NewPublisher(rocketSrc, topic)
		con := rocketmq.NewConsumer(rocketSrc, nil)
		d.mqPublisher = pub
		d.mqConsumer = con
		d.mqHealthy = pub.Healthy
	}
	d.mqTopic = topic
}

// MQPublisher returns the mq.Publisher (nil when MQ is disabled).
func (d *Data) MQPublisher() mq.Publisher {
	return d.mqPublisher
}

// MQConsumer returns the mq.Consumer (nil when MQ is disabled).
func (d *Data) MQConsumer() mq.Consumer {
	return d.mqConsumer
}

// MQTopic returns the configured topic name (empty when MQ is disabled).
func (d *Data) MQTopic() string {
	return d.mqTopic
}

// MQEnabled reports whether MQ has been configured and initialized.
func (d *Data) MQEnabled() bool {
	return d.mqPublisher != nil
}

// MQHealthy checks liveness of the MQ publisher.
// Returns nil immediately when MQ is disabled so that disabled MQ never
// contributes to a readiness failure.
// It satisfies resource.Check and can be conditionally registered into
// resource.Registry:
//
//	if d.MQEnabled() { reg.Register("mq", d.MQHealthy) }
func (d *Data) MQHealthy(ctx context.Context) error {
	if d.mqHealthy == nil {
		return nil
	}
	return d.mqHealthy(ctx)
}
