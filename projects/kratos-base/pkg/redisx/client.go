// Package redisx provides a thin wrapper around go-redis UniversalClient with
// sensible defaults, a connection-health check on Open, and a config fingerprint
// for change detection. It mirrors the shape of pkg/pgxpool for consistency.
package redisx

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds connection and pool parameters for a Redis client.
type Config struct {
	// Addrs is the list of Redis addresses. A single entry selects standalone
	// mode; multiple entries select cluster mode.
	Addrs        []string
	Username     string
	Password     string
	DB           int
	PoolSize     int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	EnableTLS    bool
}

// options maps Config to redis.UniversalOptions with sensible defaults for
// zero values. When EnableTLS is true a TLS config requiring at least TLS 1.2
// is set.
func (c Config) options() *redis.UniversalOptions {
	dialTimeout := c.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}

	opts := &redis.UniversalOptions{
		Addrs:        c.Addrs,
		Username:     c.Username,
		Password:     c.Password,
		DB:           c.DB,
		MaxRetries:   c.MaxRetries,
		DialTimeout:  dialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
	}

	if c.PoolSize > 0 {
		opts.PoolSize = c.PoolSize
	}

	if c.EnableTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return opts
}

// NewUniversalClient constructs a redis.UniversalClient from cfg without
// pinging the server. It is exposed so callers can inspect the concrete type
// (e.g. in tests) without requiring a live Redis instance.
func NewUniversalClient(cfg Config) redis.UniversalClient {
	return redis.NewUniversalClient(cfg.options())
}

// Open builds a UniversalClient and verifies connectivity with a PING. If the
// ping fails the client is closed and an error is returned.
//
// The ping honors the caller's ctx: its deadline is min(ctx deadline,
// DialTimeout). This matters on the resource.Provider rebuild path — a config
// change to an unreachable address must not block a readyz request past its own
// deadline; cancelling ctx returns promptly instead of waiting out DialTimeout.
func Open(ctx context.Context, cfg Config) (redis.UniversalClient, error) {
	client := redis.NewUniversalClient(cfg.options())

	timeout := cfg.DialTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisx: ping: %w", err)
	}
	return client, nil
}

// Fingerprint returns a SHA-256 hex digest of the connection-relevant fields.
// Two Config values with identical settings always produce the same fingerprint;
// any change to Addrs, Username, Password, DB, PoolSize, MaxRetries, timeouts,
// or EnableTLS will change the fingerprint.
func (c Config) Fingerprint() string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%d|%d|%d|%s|%s|%s|%v",
		strings.Join(c.Addrs, ","),
		c.Username,
		c.Password,
		c.DB,
		c.PoolSize,
		c.MaxRetries,
		c.DialTimeout,
		c.ReadTimeout,
		c.WriteTimeout,
		c.EnableTLS,
	)
	return hex.EncodeToString(h.Sum(nil))
}

// Ping is a convenience helper for health-check use by data-layer adapters.
func Ping(ctx context.Context, client redis.UniversalClient) error {
	return client.Ping(ctx).Err()
}
