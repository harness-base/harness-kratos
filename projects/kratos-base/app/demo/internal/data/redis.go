// Package data – redis adapter for the demo service.
// Wires a redis.UniversalClient into resource.Provider using pkg/redisx.
package data

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/z-mate/kratos-base/pkg/redisx"
	"github.com/z-mate/kratos-base/pkg/resource"
)

// Redis returns the current redis.UniversalClient, building one if none exists yet.
// Callers should treat errors as transient and wrap them with errs.DBUnavailable.
func (d *Data) Redis(ctx context.Context) (redis.UniversalClient, error) {
	return d.redisConn.Get(ctx)
}

// RedisHealthy drives a refresh and then PINGs the Redis server.
// It satisfies the resource.Check signature and can be registered directly into
// a resource.Registry:
//
//	reg.Register("redis", d.RedisHealthy)
func (d *Data) RedisHealthy(ctx context.Context) error {
	return d.redisConn.Healthy(ctx)
}

// newRedisProvider builds the resource.Provider[redis.UniversalClient] from the
// supplied selectRedis selector.
func newRedisProvider(src resource.Source, selectRedis func(any) (redisx.Config, error)) *resource.Provider[redis.UniversalClient] {
	ad := resource.Adapter[redis.UniversalClient]{
		Build: func(ctx context.Context, cfg any) (redis.UniversalClient, error) {
			rcfg, err := selectRedis(cfg)
			if err != nil {
				return nil, err
			}
			return redisx.Open(ctx, rcfg)
		},
		Close: func(c redis.UniversalClient) error {
			return c.Close()
		},
		Fingerprint: func(cfg any) string {
			rcfg, err := selectRedis(cfg)
			if err != nil {
				return ""
			}
			return rcfg.Fingerprint()
		},
		Health: func(ctx context.Context, c redis.UniversalClient) error {
			return redisx.Ping(ctx, c)
		},
	}
	return resource.New(src, ad)
}
