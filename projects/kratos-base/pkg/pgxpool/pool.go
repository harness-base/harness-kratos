// Package pgxpool provides a thin wrapper around database/sql using the pgx
// stdlib driver, with sensible pool defaults and a connection-health check on
// Open.
package pgxpool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" driver
)

// PoolConfig holds connection-string and pool tuning parameters.
type PoolConfig struct {
	DSN             string
	MaxOpen         int
	MaxIdle         int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
}

// Open opens a *sql.DB with the pgx stdlib driver, applies pool parameters
// (using sensible defaults for zero values), then pings the server within
// min(ctx deadline, ConnectTimeout) (default 5 s). If the ping fails the db is
// closed and the error is returned.
//
// The ping honors the caller's ctx so a config change to an unreachable DSN on
// the resource.Provider rebuild path does not block a readyz request past its
// own deadline; cancelling ctx returns promptly instead of waiting out
// ConnectTimeout.
func Open(ctx context.Context, cfg PoolConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: sql.Open: %w", err)
	}

	// Apply pool parameters, falling back to reasonable defaults.
	maxOpen := cfg.MaxOpen
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := cfg.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 5
	}
	connMaxLifetime := cfg.ConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = 30 * time.Minute
	}
	connMaxIdleTime := cfg.ConnMaxIdleTime
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 5 * time.Minute
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	timeout := cfg.ConnectTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err = db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pgxpool: ping: %w", err)
	}
	return db, nil
}

// Fingerprint returns a SHA-256 hex digest of the connection-relevant fields
// (DSN + pool parameters). Two PoolConfig values with identical settings
// always produce the same fingerprint.
func (c PoolConfig) Fingerprint() string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s|%d|%d|%s|%s|%s",
		c.DSN,
		c.MaxOpen,
		c.MaxIdle,
		c.ConnMaxLifetime,
		c.ConnMaxIdleTime,
		c.ConnectTimeout,
	)
	return hex.EncodeToString(h.Sum(nil))
}
