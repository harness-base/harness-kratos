// Package data wires the persistence layer for the demo service.
// It uses resource.Provider to lazily create (and re-create on config change)
// an ent.Client backed by a pgx connection pool, and a redis.UniversalClient
// backed by pkg/redisx.
package data

import (
	"context"
	"database/sql"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/redis/go-redis/v9"

	"github.com/z-mate/kratos-base/app/demo/internal/data/ent"
	"github.com/z-mate/kratos-base/pkg/mq"
	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
	"github.com/z-mate/kratos-base/pkg/redisx"
	"github.com/z-mate/kratos-base/pkg/resource"

	"entgo.io/ent/dialect"
)

// entConn bundles the ent client together with the underlying *sql.DB so the
// health probe can use db.PingContext without reaching through ent internals.
type entConn struct {
	client *ent.Client
	db     *sql.DB
}

// Data is the data-layer handle. Obtain an ent.Client via Ent; check liveness
// via Healthy; obtain a Redis client via Redis; release resources via Close.
type Data struct {
	conn      *resource.Provider[*entConn]
	redisConn *resource.Provider[redis.UniversalClient]

	// MQ fields — nil when MQ is disabled (kind == "").
	mqPublisher mq.Publisher
	mqConsumer  mq.Consumer
	mqTopic     string
	mqHealthy   func(ctx context.Context) error
}

// New creates a Data whose connections are built lazily on first use.
//
// src is a resource.Source that supplies the current configuration snapshot.
// selectPool extracts a pgxpool.PoolConfig from the snapshot value; it
// decouples Data from any concrete config type (tests supply a stub).
// selectRedis extracts a redisx.Config from the snapshot value; similarly
// decoupled for testability.
// selectMQ extracts the MQ selection from the snapshot value; when kind is ""
// MQ is left disabled (publisher/consumer remain nil, readiness check silent).
func New(
	src resource.Source,
	selectPool func(cfg any) (pgxpool.PoolConfig, error),
	selectRedis func(cfg any) (redisx.Config, error),
	selectMQ func(cfg any) (string, rabbitmq.Config, rocketmq.Config, string, error),
) *Data {
	ad := resource.Adapter[*entConn]{
		Build: func(ctx context.Context, cfg any) (*entConn, error) {
			pcfg, err := selectPool(cfg)
			if err != nil {
				return nil, fmt.Errorf("data: selectPool: %w", err)
			}
			db, err := pgxpool.Open(ctx, pcfg)
			if err != nil {
				return nil, fmt.Errorf("data: open pool: %w", err)
			}
			drv := entsql.OpenDB(dialect.Postgres, db)
			client := ent.NewClient(ent.Driver(drv))
			return &entConn{client: client, db: db}, nil
		},
		Close: func(c *entConn) error {
			return c.client.Close()
		},
		Fingerprint: func(cfg any) string {
			pcfg, err := selectPool(cfg)
			if err != nil {
				return ""
			}
			return pcfg.Fingerprint()
		},
		Health: func(ctx context.Context, c *entConn) error {
			return c.db.PingContext(ctx)
		},
	}
	d := &Data{
		conn:      resource.New(src, ad),
		redisConn: newRedisProvider(src, selectRedis),
	}
	initMQ(d, src, selectMQ)
	return d
}

// Ent returns the current ent.Client, building one if none exists yet.
func (d *Data) Ent(ctx context.Context) (*ent.Client, error) {
	c, err := d.conn.Get(ctx)
	if err != nil {
		return nil, err
	}
	return c.client, nil
}

// Healthy drives a refresh and then pings the underlying database. It
// satisfies the resource.Check signature and can be registered directly into
// a resource.Registry:
//
//	reg.Register("postgres", d.Healthy)
func (d *Data) Healthy(ctx context.Context) error {
	return d.conn.Healthy(ctx)
}

// Close tears down the connection pool, ent client, Redis client, and MQ.
//
// The PG/Redis providers may be nil for MQ-only Data values (e.g. built via the
// test helper NewForTest); Close treats a nil provider as nothing to release so
// it is always safe to call on any Data handle the lifecycle hands it.
func (d *Data) Close() error {
	var pgErr, redisErr error
	if d.conn != nil {
		pgErr = d.conn.Close()
	}
	if d.redisConn != nil {
		redisErr = d.redisConn.Close()
	}
	var mqErr error
	if d.mqPublisher != nil {
		mqErr = d.mqPublisher.Close()
	}
	if d.mqConsumer != nil {
		_ = d.mqConsumer.Close()
	}
	if pgErr != nil {
		return pgErr
	}
	if redisErr != nil {
		return redisErr
	}
	return mqErr
}
