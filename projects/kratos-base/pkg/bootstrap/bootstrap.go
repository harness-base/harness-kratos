// Package bootstrap handles the two-phase config strategy for kratos-base.
//
// Phase 1 (bootstrap): read a small bootstrap.yaml that tells us which config
// source to use (local file, nacos, etcd, k8s).  Resolved once at startup.
//
// Phase 2 (runtime): NewConfigSource returns a live kratos config.Config that
// can be watched for hot-reload (see pkg/confcenter for the Manager layer).
package bootstrap

import (
	"fmt"
	"os"

	etcdcfg "github.com/go-kratos/kratos/contrib/config/etcd/v2"
	k8scfg "github.com/go-kratos/kratos/contrib/config/kubernetes/v2"
	nacoscfg "github.com/go-kratos/kratos/contrib/config/nacos/v2"
	kratosconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"gopkg.in/yaml.v3"

	"github.com/z-mate/kratos-base/pkg/backends"
)

// Bootstrap is the top-level structure of bootstrap.yaml.
type Bootstrap struct {
	Infra InfraConfig `yaml:"infra" json:"infra"`
}

// InfraConfig specifies the runtime config source.
type InfraConfig struct {
	// Mode selects the config backend: local | file | etcd | nacos | k8s.
	Mode string `yaml:"mode" json:"mode"`
	// Path is the runtime config file path used in "local" / "file" mode.
	Path string `yaml:"path" json:"path"`
	// Etcd holds connection details for the etcd config backend.
	Etcd backends.EtcdConfig `yaml:"etcd" json:"etcd"`
	// Nacos holds connection details for the Nacos config backend.
	Nacos backends.NacosConfig `yaml:"nacos" json:"nacos"`
	// K8s holds connection details for the Kubernetes ConfigMap config backend.
	K8s backends.K8sConfig `yaml:"k8s" json:"k8s"`
	// Registry holds service registry configuration.
	Registry RegistryConfig `yaml:"registry" json:"registry"`
}

// RegistryConfig specifies the service registry backend for service
// registration and discovery.
type RegistryConfig struct {
	// Kind selects the registry backend: local | etcd | nacos | k8s.
	// "local" (the default) disables registration entirely — suitable for
	// single-node development where services communicate via direct addresses.
	Kind string `yaml:"kind" json:"kind"`
	// Advertise is the gRPC address this instance advertises to the registry,
	// e.g. "127.0.0.1:9000".  Empty means the address is inferred from the
	// runtime server.grpc.addr at startup time.
	Advertise string `yaml:"advertise" json:"advertise"`
}

// Load reads and parses a bootstrap.yaml file at path.
// If the INFRA_MODE environment variable is non-empty its value overrides
// Bootstrap.Infra.Mode after parsing.
func Load(path string) (Bootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Bootstrap{}, fmt.Errorf("bootstrap: read %q: %w", path, err)
	}

	var bs Bootstrap
	if err := yaml.Unmarshal(data, &bs); err != nil {
		return Bootstrap{}, fmt.Errorf("bootstrap: parse %q: %w", path, err)
	}

	if mode := os.Getenv("INFRA_MODE"); mode != "" {
		bs.Infra.Mode = mode
	}

	return bs, nil
}

// NewConfigSource creates a kratos config.Config for the runtime phase based on
// the bootstrap settings.
//
// Supported modes:
//   - "local" / "file"  → file-backed config.Config (supports Watch).
//   - "etcd"            → etcd-backed config source via contrib/config/etcd.
//   - "nacos"           → Nacos-backed config source via contrib/config/nacos.
//   - "k8s"             → Kubernetes ConfigMap source via contrib/config/kubernetes.
//   - anything else     → returns an error.
//
// Configuration is "eager-load": Load() is expected to be called immediately
// by the caller; failure is fatal at startup (fail-fast).
func NewConfigSource(bs Bootstrap) (kratosconfig.Config, error) {
	switch bs.Infra.Mode {
	case "local", "file":
		c := kratosconfig.New(
			kratosconfig.WithSource(file.NewSource(bs.Infra.Path)),
		)
		return c, nil

	case "etcd":
		client, err := backends.NewEtcdClient(bs.Infra.Etcd)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: etcd backend: %w", err)
		}
		src, err := etcdcfg.New(client, etcdcfg.WithPath(bs.Infra.Path))
		if err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("bootstrap: etcd source: %w", err)
		}
		return kratosconfig.New(kratosconfig.WithSource(src)), nil

	case "nacos":
		nacosClient, err := backends.NewNacosConfigClient(bs.Infra.Nacos)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: nacos backend: %w", err)
		}
		src := nacoscfg.NewConfigSource(
			nacosClient,
			nacoscfg.WithGroup(bs.Infra.Nacos.Group),
			nacoscfg.WithDataID(bs.Infra.Nacos.DataID),
		)
		return kratosconfig.New(kratosconfig.WithSource(src)), nil

	case "k8s":
		k := bs.Infra.K8s
		// Name is required: it becomes a metadata.name FieldSelector that scopes
		// the load to a single ConfigMap.  Without it the contrib source Lists
		// EVERY ConfigMap in the namespace and merges all their data keys —
		// unpredictable config with no error — so fail fast instead.
		if k.Name == "" {
			return nil, fmt.Errorf("bootstrap: k8s: name must not be empty")
		}
		opts := []k8scfg.Option{
			k8scfg.Namespace(k.Namespace),
			// FieldSelector selects the ConfigMap by name.
			k8scfg.FieldSelector("metadata.name=" + k.Name),
		}
		// KubeConfig receives the kubeconfig *file path* (as verified by go doc:
		// contrib passes it directly to clientcmd.BuildConfigFromFlags).
		// Empty string causes the source to fall back to in-cluster config.
		if k.KubeconfigPath != "" {
			// Validate the path exists early so we get a clear error at startup.
			if _, err := os.Stat(k.KubeconfigPath); err != nil {
				return nil, fmt.Errorf("bootstrap: k8s: kubeconfig %q: %w", k.KubeconfigPath, err)
			}
			opts = append(opts, k8scfg.KubeConfig(k.KubeconfigPath))
		}
		src := k8scfg.NewSource(opts...)
		return kratosconfig.New(kratosconfig.WithSource(src)), nil

	default:
		return nil, fmt.Errorf("bootstrap: unknown config source mode %q", bs.Infra.Mode)
	}
}
