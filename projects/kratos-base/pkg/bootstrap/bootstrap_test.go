package bootstrap_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/z-mate/kratos-base/pkg/backends"
	"github.com/z-mate/kratos-base/pkg/bootstrap"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	// Note: not parallel at the top level because subtests that call t.Setenv
	// cannot be run in parallel (Go runtime prohibits it).

	cases := []struct {
		name     string
		yaml     string
		envMode  string
		wantMode string
		wantPath string
		wantErr  bool
	}{
		{
			name: "reads fields correctly",
			yaml: `
infra:
  mode: local
  path: configs/runtime.yaml
`,
			wantMode: "local",
			wantPath: "configs/runtime.yaml",
		},
		{
			name: "env INFRA_MODE overrides mode",
			yaml: `
infra:
  mode: local
  path: configs/runtime.yaml
`,
			envMode:  "nacos",
			wantMode: "nacos",
			wantPath: "configs/runtime.yaml",
		},
		{
			name: "empty INFRA_MODE does not override",
			yaml: `
infra:
  mode: etcd
  path: /some/path.yaml
`,
			envMode:  "",
			wantMode: "etcd",
			wantPath: "/some/path.yaml",
		},
		{
			name: "env INFRA_MODE overrides to k8s",
			yaml: `
infra:
  mode: local
  path: /some/path.yaml
`,
			envMode:  "k8s",
			wantMode: "k8s",
			wantPath: "/some/path.yaml",
		},
		{
			name:    "missing file returns error",
			yaml:    "",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Subtests that use t.Setenv must NOT call t.Parallel().
			var path string
			if tc.wantErr {
				// Point to a path that doesn't exist.
				path = filepath.Join(t.TempDir(), "nonexistent.yaml")
			} else {
				path = writeTempYAML(t, tc.yaml)
			}

			if tc.envMode != "" {
				t.Setenv("INFRA_MODE", tc.envMode)
			}

			bs, err := bootstrap.Load(path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if bs.Infra.Mode != tc.wantMode {
				t.Errorf("Mode = %q, want %q", bs.Infra.Mode, tc.wantMode)
			}
			if bs.Infra.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q", bs.Infra.Path, tc.wantPath)
			}
		})
	}
}

func TestNewConfigSource(t *testing.T) {
	t.Parallel()

	// Create a temp runtime.yaml so the local source has a real file to open.
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(runtimePath, []byte("server:\n  grpc:\n    addr: \":9000\"\n"), 0o600); err != nil {
		t.Fatalf("write runtime yaml: %v", err)
	}

	cases := []struct {
		name        string
		mode        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name: "local mode returns config",
			mode: "local",
			path: runtimePath,
		},
		{
			name: "file alias mode returns config",
			mode: "file",
			path: runtimePath,
		},
		{
			name:        "unknown mode returns error",
			mode:        "consul",
			wantErr:     true,
			errContains: "consul",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bs := bootstrap.Bootstrap{
				Infra: bootstrap.InfraConfig{
					Mode: tc.mode,
					Path: tc.path,
				},
			}
			cfg, err := bootstrap.NewConfigSource(bs)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errContains)
				}
				if tc.errContains != "" {
					if got := err.Error(); !contains(got, tc.errContains) {
						t.Errorf("error = %q, want it to contain %q", got, tc.errContains)
					}
				}
				if cfg != nil {
					t.Error("expected nil config on error, got non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewConfigSource() unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config, got nil")
			}
			// Ensure Load works for local/file mode.
			if err := cfg.Load(); err != nil {
				t.Fatalf("cfg.Load() error: %v", err)
			}
			_ = cfg.Close()
		})
	}
}

// ---------------------------------------------------------------------------
// Etcd branch: unreachable endpoint → error without hanging
// ---------------------------------------------------------------------------

func TestNewConfigSource_EtcdUnreachable(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Mode: "etcd",
			Path: "/config/test",
			Etcd: backends.EtcdConfig{
				Endpoints:   []string{"127.0.0.1:12379"}, // nothing listening
				DialTimeout: 500 * time.Millisecond,
			},
		},
	}
	// NewEtcdClient probes the endpoint with DialTimeout; error should surface
	// inside NewConfigSource (backend construction), well within 2s.
	cfg, err := bootstrap.NewConfigSource(bs)
	if err == nil {
		_ = cfg.Close()
		t.Fatal("expected error for unreachable etcd endpoint, got nil")
	}
	if cfg != nil {
		_ = cfg.Close()
		t.Error("expected nil config on error, got non-nil")
	}
	t.Logf("etcd error (expected): %v", err)
}

// ---------------------------------------------------------------------------
// Nacos branch: unreachable server
// ---------------------------------------------------------------------------

func TestNewConfigSource_NacosUnreachable(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Mode: "nacos",
			Nacos: backends.NacosConfig{
				ServerAddrs: []string{"127.0.0.1:18848"}, // nothing listening
				Group:       "DEFAULT_GROUP",
				DataID:      "test.yaml",
				TimeoutMs:   300, // very short so failure is fast
			},
		},
	}
	// nacos-sdk-go v1 does not probe the server during client construction.
	// NewConfigSource itself will succeed (returning a config.Config).
	// The error surfaces on cfg.Load() — that is the correct assertion point.
	cfg, constructErr := bootstrap.NewConfigSource(bs)
	if constructErr != nil {
		// The SDK returned an error at construction — that's acceptable too.
		t.Logf("nacos construction error (acceptable): %v", constructErr)
		return
	}
	if cfg == nil {
		t.Fatal("expected non-nil config when nacos construction succeeds")
	}

	// Load() must return an error for the unreachable server.
	loadErr := cfg.Load()
	_ = cfg.Close()
	if loadErr == nil {
		t.Fatal("expected error from cfg.Load() for unreachable nacos server, got nil")
	}
	t.Logf("nacos Load() error (expected): %v", loadErr)
}

// ---------------------------------------------------------------------------
// K8s branch: non-existent kubeconfig path → error at NewConfigSource
// ---------------------------------------------------------------------------

func TestNewConfigSource_K8sNonExistentKubeconfig(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Mode: "k8s",
			K8s: backends.K8sConfig{
				Namespace:      "default",
				Name:           "my-config",
				KubeconfigPath: "/nonexistent/path/to/kubeconfig",
			},
		},
	}
	cfg, err := bootstrap.NewConfigSource(bs)
	if err == nil {
		_ = cfg.Close()
		t.Fatal("expected error for non-existent kubeconfig path, got nil")
	}
	if cfg != nil {
		_ = cfg.Close()
		t.Error("expected nil config on error, got non-nil")
	}
	t.Logf("k8s kubeconfig error (expected): %v", err)
}

// ---------------------------------------------------------------------------
// K8s branch: empty kubeconfig (in-cluster) → NewConfigSource succeeds
// (no cluster available so Load() would fail, but construction must not error)
// ---------------------------------------------------------------------------

func TestNewConfigSource_K8sInClusterConstruction(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Mode: "k8s",
			K8s: backends.K8sConfig{
				Namespace:      "default",
				Name:           "my-config", // required so we exercise the success path
				KubeconfigPath: "",          // empty = in-cluster
			},
		},
	}
	// Construction must succeed — the source only builds the REST client lazily.
	cfg, err := bootstrap.NewConfigSource(bs)
	if err != nil {
		t.Fatalf("expected successful construction for in-cluster k8s config, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// We don't call Load() here because there's no cluster in the test environment.
	_ = cfg.Close()
}

// ---------------------------------------------------------------------------
// K8s branch: empty Name → fail fast (R11F8)
//
// An empty ConfigMap name would otherwise make the contrib source List EVERY
// ConfigMap in the namespace and merge all their data keys — unpredictable
// config with no error. NewConfigSource must reject it up front.
// ---------------------------------------------------------------------------

func TestNewConfigSource_K8sEmptyNameErrors(t *testing.T) {
	t.Parallel()

	bs := bootstrap.Bootstrap{
		Infra: bootstrap.InfraConfig{
			Mode: "k8s",
			K8s: backends.K8sConfig{
				Namespace:      "default",
				Name:           "", // empty → must fail fast, not list whole namespace
				KubeconfigPath: "", // in-cluster; the Name check must fire before any client build
			},
		},
	}
	cfg, err := bootstrap.NewConfigSource(bs)
	if err == nil {
		_ = cfg.Close()
		t.Fatal("expected error for empty k8s ConfigMap name, got nil")
	}
	if cfg != nil {
		_ = cfg.Close()
		t.Error("expected nil config on error, got non-nil")
	}
	if !contains(err.Error(), "name must not be empty") {
		t.Errorf("error = %q, want it to mention %q", err.Error(), "name must not be empty")
	}
}

// ---------------------------------------------------------------------------
// INFRA_MODE env-var override: five modes covered
// ---------------------------------------------------------------------------

func TestLoad_InfraModeEnvCoversModes(t *testing.T) {
	modes := []string{"local", "file", "etcd", "nacos", "k8s"}

	for _, mode := range modes {
		mode := mode
		t.Run("mode="+mode, func(t *testing.T) {
			// Can't run parallel when using t.Setenv.
			path := writeTempYAML(t, "infra:\n  mode: local\n  path: /tmp/x.yaml\n")
			t.Setenv("INFRA_MODE", mode)

			bs, err := bootstrap.Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if bs.Infra.Mode != mode {
				t.Errorf("Mode = %q, want %q", bs.Infra.Mode, mode)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RegistryConfig YAML/JSON round-trip
// ---------------------------------------------------------------------------

func TestRegistryConfigYAML(t *testing.T) {
	t.Parallel()

	raw := `
infra:
  mode: local
  path: configs/runtime.yaml
  registry:
    kind: etcd
    advertise: "127.0.0.1:9000"
`
	path := writeTempYAML(t, raw)
	bs, err := bootstrap.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if bs.Infra.Registry.Kind != "etcd" {
		t.Errorf("Registry.Kind = %q, want etcd", bs.Infra.Registry.Kind)
	}
	if bs.Infra.Registry.Advertise != "127.0.0.1:9000" {
		t.Errorf("Registry.Advertise = %q, want 127.0.0.1:9000", bs.Infra.Registry.Advertise)
	}
}

func TestRegistryConfigDefaultLocal(t *testing.T) {
	t.Parallel()

	// Omitting the registry section should default to zero (kind="", local).
	raw := `
infra:
  mode: local
  path: configs/runtime.yaml
`
	path := writeTempYAML(t, raw)
	bs, err := bootstrap.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if bs.Infra.Registry.Kind != "" {
		t.Errorf("Registry.Kind = %q, want empty (local)", bs.Infra.Registry.Kind)
	}
}

func TestRegistryConfigAllKindsParseOK(t *testing.T) {
	t.Parallel()

	kinds := []string{"local", "etcd", "nacos", "k8s"}
	for _, kind := range kinds {
		kind := kind
		t.Run("kind="+kind, func(t *testing.T) {
			t.Parallel()

			raw := "infra:\n  mode: local\n  path: /tmp/r.yaml\n  registry:\n    kind: " + kind + "\n"
			path := writeTempYAML(t, raw)
			bs, err := bootstrap.Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if bs.Infra.Registry.Kind != kind {
				t.Errorf("Registry.Kind = %q, want %q", bs.Infra.Registry.Kind, kind)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
