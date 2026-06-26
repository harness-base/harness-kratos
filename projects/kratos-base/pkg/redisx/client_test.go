package redisx_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/z-mate/kratos-base/pkg/redisx"
)

// TestFingerprint_Stability verifies that the same config always yields the
// same fingerprint.
func TestFingerprint_Stability(t *testing.T) {
	cfg := redisx.Config{
		Addrs:        []string{"localhost:6379"},
		Username:     "user",
		Password:     "pass",
		DB:           0,
		PoolSize:     10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		EnableTLS:    false,
	}

	fp1 := cfg.Fingerprint()
	fp2 := cfg.Fingerprint()
	if fp1 != fp2 {
		t.Fatalf("same config produced different fingerprints: %q vs %q", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatal("Fingerprint returned empty string")
	}
}

// TestFingerprint_ChangeFields verifies that changing connection-relevant fields
// changes the fingerprint.
func TestFingerprint_ChangeFields(t *testing.T) {
	base := redisx.Config{
		Addrs:    []string{"localhost:6379"},
		Password: "pass",
		DB:       0,
		PoolSize: 10,
	}

	tests := []struct {
		name   string
		mutate func(c redisx.Config) redisx.Config
	}{
		{
			name: "change Addrs",
			mutate: func(c redisx.Config) redisx.Config {
				c.Addrs = []string{"localhost:6380"}
				return c
			},
		},
		{
			name: "change DB",
			mutate: func(c redisx.Config) redisx.Config {
				c.DB = 2
				return c
			},
		},
		{
			name: "change PoolSize",
			mutate: func(c redisx.Config) redisx.Config {
				c.PoolSize = 20
				return c
			},
		},
		{
			name: "change Password",
			mutate: func(c redisx.Config) redisx.Config {
				c.Password = "other"
				return c
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changed := tc.mutate(base)
			if base.Fingerprint() == changed.Fingerprint() {
				t.Fatalf("%s: expected different fingerprint but got same", tc.name)
			}
		})
	}
}

// TestModeSelection verifies that a single addr produces *redis.Client and
// multiple addrs produce *redis.ClusterClient. No real Redis needed — we only
// construct the client without pinging.
func TestModeSelection(t *testing.T) {
	tests := []struct {
		name     string
		addrs    []string
		wantType string
	}{
		{
			name:     "single addr -> standalone client",
			addrs:    []string{"127.0.0.1:6379"},
			wantType: "*redis.Client",
		},
		{
			name:     "multiple addrs -> cluster client",
			addrs:    []string{"127.0.0.1:7000", "127.0.0.1:7001", "127.0.0.1:7002"},
			wantType: "*redis.ClusterClient",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := redisx.Config{
				Addrs:    tc.addrs,
				Password: "",
				DB:       0,
			}
			client := redisx.NewUniversalClient(cfg)
			defer func() { _ = client.Close() }()

			switch tc.wantType {
			case "*redis.Client":
				if _, ok := client.(*redis.Client); !ok {
					t.Fatalf("expected *redis.Client, got %T", client)
				}
			case "*redis.ClusterClient":
				if _, ok := client.(*redis.ClusterClient); !ok {
					t.Fatalf("expected *redis.ClusterClient, got %T", client)
				}
			default:
				t.Fatalf("unknown wantType %q", tc.wantType)
			}
		})
	}
}

// TestOpen_Unreachable verifies that Open returns promptly (within ~2s of the
// timeout) when the server is not reachable. No real Redis needed.
func TestOpen_Unreachable(t *testing.T) {
	cfg := redisx.Config{
		Addrs:       []string{"127.0.0.1:1"},
		DialTimeout: time.Second,
	}

	start := time.Now()
	client, err := redisx.Open(context.Background(), cfg)
	elapsed := time.Since(start)

	if err == nil {
		_ = client.Close()
		t.Fatal("expected an error for unreachable addr, got nil")
	}

	// Allow 2× for CI scheduling jitter.
	const maxElapsed = 2 * time.Second
	if elapsed > maxElapsed {
		t.Fatalf("Open took %v, expected < %v (should respect DialTimeout)", elapsed, maxElapsed)
	}
	t.Logf("Open returned in %v with error: %v", elapsed, err)
}

// TestOpen_HonorsCanceledContext verifies Open returns promptly when the
// caller's ctx is already canceled, even with a long DialTimeout — proving the
// ping derives its deadline from ctx, not just DialTimeout. This is the behavior
// the resource.Provider rebuild path relies on so a bad-addr config change
// cannot block a readyz request past its own deadline.
func TestOpen_HonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	cfg := redisx.Config{
		Addrs:       []string{"127.0.0.1:1"},
		DialTimeout: 30 * time.Second, // long: without ctx-awareness Open would block this long
	}

	start := time.Now()
	client, err := redisx.Open(ctx, cfg)
	elapsed := time.Since(start)

	if err == nil {
		_ = client.Close()
		t.Fatal("expected an error for canceled ctx, got nil")
	}
	const maxElapsed = 2 * time.Second
	if elapsed > maxElapsed {
		t.Fatalf("Open took %v with canceled ctx, expected < %v (must honor caller ctx, not DialTimeout)", elapsed, maxElapsed)
	}
	t.Logf("Open honored canceled ctx, returned in %v with error: %v", elapsed, err)
}
