// export_test.go exposes internal helpers for use in black-box tests (_test package).
// File is compiled only during `go test` (same package, not _test).
package data

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/z-mate/kratos-base/app/demo/internal/data/ent"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// fixedEntSource is a resource.Source carrying a fixed snapshot. Used only by
// NewWithEntClient to back an *entConn provider that always returns the same
// client (no rebuild, no real Postgres).
type fixedEntSource struct{}

func (fixedEntSource) Current() resource.Snapshot { return resource.Snapshot{Version: 1} }

// NewWithEntClient creates a *Data whose Ent(ctx) returns the supplied ent
// client (e.g. a sqlite-backed enttest client). The connection provider is
// wired so Build always yields the injected client; no Postgres is needed.
// Redis and MQ are left disabled. Intended for repo tests that need a real,
// queryable ent.Client to exercise hit/miss/error contracts.
//
// db is left nil on the underlying entConn — callers must not invoke Healthy
// (which pings db). Get/query paths use only the client.
func NewWithEntClient(client *ent.Client) *Data {
	ad := resource.Adapter[*entConn]{
		Build: func(_ context.Context, _ any) (*entConn, error) {
			return &entConn{client: client}, nil
		},
	}
	return &Data{conn: resource.New(fixedEntSource{}, ad)}
}

// NewWithRedisClient creates a *Data whose Redis(ctx) always returns the
// supplied redis.UniversalClient (no dial, no real server). The PG provider is
// left nil — callers must not call Ent/Healthy. Intended for HitsRepo tests that
// need to observe the exact command (and therefore the key) HitsRepo.Incr sends,
// by attaching a redis.Hook to the injected client.
func NewWithRedisClient(client redis.UniversalClient) *Data {
	ad := resource.Adapter[redis.UniversalClient]{
		Build: func(_ context.Context, _ any) (redis.UniversalClient, error) {
			return client, nil
		},
	}
	return &Data{redisConn: resource.New(fixedEntSource{}, ad)}
}

// NewForTest creates a minimal *Data with the given MQ publisher, consumer, and
// topic injected directly — bypassing resource.Provider machinery. The PG and
// Redis providers are set to nil (not usable), so callers must not call Ent or
// Redis on the returned Data.
//
// Intended for unit tests that need to control the MQ publisher without a real
// broker.
func NewForTest(pub mq.Publisher, con mq.Consumer, topic string) *Data {
	d := &Data{}
	d.mqPublisher = pub
	d.mqConsumer = con
	d.mqTopic = topic
	if pub != nil {
		if hc, ok := pub.(interface {
			Healthy(ctx context.Context) error
		}); ok {
			d.mqHealthy = hc.Healthy
		}
	}
	return d
}
