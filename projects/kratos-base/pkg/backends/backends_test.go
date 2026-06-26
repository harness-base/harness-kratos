package backends_test

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/z-mate/kratos-base/pkg/backends"
)

// ---------------------------------------------------------------------------
// YAML / JSON round-trip tests for config structs
// ---------------------------------------------------------------------------

func TestEtcdConfigYAML(t *testing.T) {
	t.Parallel()

	raw := `
endpoints:
  - "etcd1:2379"
  - "etcd2:2379"
username: alice
password: secret
dial_timeout: 3s
`
	var cfg backends.EtcdConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if len(cfg.Endpoints) != 2 {
		t.Errorf("Endpoints len = %d, want 2", len(cfg.Endpoints))
	}
	if cfg.Endpoints[0] != "etcd1:2379" {
		t.Errorf("Endpoints[0] = %q, want %q", cfg.Endpoints[0], "etcd1:2379")
	}
	if cfg.Username != "alice" {
		t.Errorf("Username = %q, want %q", cfg.Username, "alice")
	}
	if cfg.DialTimeout != 3*time.Second {
		t.Errorf("DialTimeout = %v, want 3s", cfg.DialTimeout)
	}
}

func TestEtcdConfigJSON(t *testing.T) {
	t.Parallel()

	raw := `{"endpoints":["etcd1:2379"],"username":"bob","password":"pw","dial_timeout":2000000000}`
	var cfg backends.EtcdConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if cfg.Username != "bob" {
		t.Errorf("Username = %q, want bob", cfg.Username)
	}
	if cfg.DialTimeout != 2*time.Second {
		t.Errorf("DialTimeout = %v, want 2s", cfg.DialTimeout)
	}
}

func TestNacosConfigYAML(t *testing.T) {
	t.Parallel()

	raw := `
server_addrs:
  - "nacos:8848"
namespace: "test-ns"
group: "DEFAULT_GROUP"
data_id: "runtime.yaml"
username: "nacos"
password: "nacos"
timeout_ms: 5000
`
	var cfg backends.NacosConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if len(cfg.ServerAddrs) != 1 {
		t.Errorf("ServerAddrs len = %d, want 1", len(cfg.ServerAddrs))
	}
	if cfg.Namespace != "test-ns" {
		t.Errorf("Namespace = %q, want test-ns", cfg.Namespace)
	}
	if cfg.Group != "DEFAULT_GROUP" {
		t.Errorf("Group = %q, want DEFAULT_GROUP", cfg.Group)
	}
	if cfg.DataID != "runtime.yaml" {
		t.Errorf("DataID = %q, want runtime.yaml", cfg.DataID)
	}
	if cfg.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs = %d, want 5000", cfg.TimeoutMs)
	}
}

func TestNacosConfigJSON(t *testing.T) {
	t.Parallel()

	raw := `{"server_addrs":["nacos:8848"],"namespace":"","group":"G","data_id":"d","username":"u","password":"p","timeout_ms":3000}`
	var cfg backends.NacosConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if cfg.Group != "G" {
		t.Errorf("Group = %q, want G", cfg.Group)
	}
	if cfg.TimeoutMs != 3000 {
		t.Errorf("TimeoutMs = %d, want 3000", cfg.TimeoutMs)
	}
}

func TestK8sConfigYAML(t *testing.T) {
	t.Parallel()

	raw := `
namespace: "default"
name: "runtime-config"
kubeconfig_path: "/home/user/.kube/config"
`
	var cfg backends.K8sConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if cfg.Namespace != "default" {
		t.Errorf("Namespace = %q, want default", cfg.Namespace)
	}
	if cfg.Name != "runtime-config" {
		t.Errorf("Name = %q, want runtime-config", cfg.Name)
	}
	if cfg.KubeconfigPath != "/home/user/.kube/config" {
		t.Errorf("KubeconfigPath = %q, want /home/user/.kube/config", cfg.KubeconfigPath)
	}
}

func TestK8sConfigJSON(t *testing.T) {
	t.Parallel()

	raw := `{"namespace":"kube-system","name":"cfg","kubeconfig_path":""}`
	var cfg backends.K8sConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if cfg.Namespace != "kube-system" {
		t.Errorf("Namespace = %q, want kube-system", cfg.Namespace)
	}
	if cfg.KubeconfigPath != "" {
		t.Errorf("KubeconfigPath = %q, want empty", cfg.KubeconfigPath)
	}
}

// ---------------------------------------------------------------------------
// NewEtcdClient: unreachable endpoint with short timeout → error ≤ ~1s
// ---------------------------------------------------------------------------

func TestNewEtcdClient_UnreachableReturnsError(t *testing.T) {
	t.Parallel()

	// clientv3 is lazy-connect; the Status probe in NewEtcdClient will surface
	// the error.  Use a short timeout so the test finishes quickly.
	cfg := backends.EtcdConfig{
		Endpoints:   []string{"127.0.0.1:12379"}, // nothing listening
		DialTimeout: 500 * time.Millisecond,
	}
	client, err := backends.NewEtcdClient(cfg)
	if err == nil {
		_ = client.Close()
		t.Fatal("expected error for unreachable endpoint, got nil")
	}
	// Client must be nil on error.
	if client != nil {
		_ = client.Close()
		t.Error("expected nil client on error")
	}
}

func TestNewEtcdClient_EmptyEndpoints(t *testing.T) {
	t.Parallel()

	cfg := backends.EtcdConfig{Endpoints: nil}
	client, err := backends.NewEtcdClient(cfg)
	if err == nil {
		_ = client.Close()
		t.Fatal("expected error for empty endpoints, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewNacosConfigClient: unreachable server
// ---------------------------------------------------------------------------

// NewNacosConfigClient is a non-probing constructor: nacos-sdk-go v1 does NOT
// contact the server during client construction (the connection is built on the
// first Get/Watch call). The construction contract is therefore: for valid
// inputs the constructor returns a non-nil client and no error, even when the
// configured server is unreachable, and it does so without blocking on a network
// round-trip. Unreachability itself is asserted at the cfg.Load() layer in
// bootstrap_test.go (TestNewConfigSource_NacosUnreachable).
func TestNewNacosConfigClient_ConstructionSucceeds(t *testing.T) {
	t.Parallel()

	cfg := backends.NacosConfig{
		ServerAddrs: []string{"127.0.0.1:18848"}, // nothing listening
		Namespace:   "",
		Group:       "DEFAULT_GROUP",
		DataID:      "test.yaml",
		TimeoutMs:   200,
	}

	start := time.Now()
	c, err := backends.NewNacosConfigClient(cfg)
	elapsed := time.Since(start)

	// Hard contract: construction must succeed for valid inputs.
	if err != nil {
		t.Fatalf("NewNacosConfigClient: construction must not error for valid config, got: %v", err)
	}
	if c == nil {
		t.Fatal("NewNacosConfigClient: expected non-nil config client on success")
	}
	// Non-probing: the constructor must not attempt a network round-trip to the
	// unreachable server. Allow generous slack for slow CI, but a TCP
	// connect/dial probe to a dead port plus retries would blow far past this.
	if elapsed > 2*time.Second {
		t.Errorf("NewNacosConfigClient took %v; constructor must not probe the server (expected lazy/non-blocking construction)", elapsed)
	}
}

func TestNewNacosConfigClient_EmptyAddrs(t *testing.T) {
	t.Parallel()

	cfg := backends.NacosConfig{ServerAddrs: nil}
	c, err := backends.NewNacosConfigClient(cfg)
	if err == nil {
		t.Fatalf("expected error for empty server_addrs, got nil (client=%v)", c)
	}
}

// ---------------------------------------------------------------------------
// NewNacosNamingClient (v2 SDK, registry use)
// ---------------------------------------------------------------------------

// NewNacosNamingClient is a non-probing constructor: nacos-sdk-go v2 is
// lazy-connect, so construction succeeds for valid inputs even when the server
// is unreachable, and it must not block on a network round-trip. The error for
// an unreachable server surfaces on the first Register/GetService call, not at
// construction.
func TestNewNacosNamingClient_ConstructionSucceeds(t *testing.T) {
	t.Parallel()

	cfg := backends.NacosConfig{
		ServerAddrs: []string{"127.0.0.1:18848"}, // nothing listening
		Namespace:   "",
		Group:       "DEFAULT_GROUP",
		TimeoutMs:   200,
	}

	start := time.Now()
	c, err := backends.NewNacosNamingClient(cfg)
	elapsed := time.Since(start)

	// Hard contract: construction must succeed for valid inputs.
	if err != nil {
		t.Fatalf("NewNacosNamingClient: construction must not error for valid config, got: %v", err)
	}
	if c == nil {
		t.Fatal("NewNacosNamingClient: expected non-nil naming client on success")
	}
	// Non-probing/lazy-connect: construction must not block on the unreachable
	// server. Generous slack for slow CI; a real dial+retry would exceed it.
	if elapsed > 2*time.Second {
		t.Errorf("NewNacosNamingClient took %v; constructor must be lazy-connect (no blocking probe at construction)", elapsed)
	}
}

func TestNewNacosNamingClient_EmptyAddrs(t *testing.T) {
	t.Parallel()

	cfg := backends.NacosConfig{ServerAddrs: nil}
	c, err := backends.NewNacosNamingClient(cfg)
	if err == nil {
		t.Fatalf("expected error for empty server_addrs, got nil (client=%v)", c)
	}
}
