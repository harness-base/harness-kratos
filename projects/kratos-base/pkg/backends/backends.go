// Package backends is the connection-detail layer for external config-source
// backends.  Each exported constructor builds and optionally probes the client;
// the rest of the application only calls these constructors and passes the
// resulting client down into the appropriate contrib config adapter.
//
// Design rules:
//   - Connection details (endpoints, credentials, kubeconfig paths) live only here.
//   - Higher layers (pkg/bootstrap) receive ready-to-use client values.
//   - For etcd: clientv3 is lazy-connect; New() rarely fails.  We attempt a
//     single Status probe with the caller-supplied DialTimeout so that
//     unreachable clusters surface early (at bootstrap time) rather than
//     silently accepting all requests.
//   - For nacos: the contrib package uses nacos-sdk-go v1 for config (IConfigClient)
//     and nacos-sdk-go v2 for registry (INamingClient); both SDKs build the
//     connection on first use, so constructors only validate input and instantiate.
//   - For k8s: the contrib kubernetes source self-builds its connection from the
//     supplied kubeconfig/in-cluster settings (see KubeConfig option); this layer
//     only carries the structural config, no client object is constructed here.
package backends

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	clientsv2 "github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	constantv2 "github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	vov2 "github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/nacos-group/nacos-sdk-go/vo"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ---------------------------------------------------------------------------
// Etcd
// ---------------------------------------------------------------------------

// EtcdConfig carries connection parameters for an etcd cluster.
type EtcdConfig struct {
	Endpoints   []string      `json:"endpoints"    yaml:"endpoints"`
	Username    string        `json:"username"     yaml:"username"`
	Password    string        `json:"password"     yaml:"password"`
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
}

// NewEtcdClientLazy constructs a clientv3.Client WITHOUT the connectivity
// probe. Use it for roles that must tolerate etcd being down at startup —
// e.g. the registry's non-fatal registration, where the connection is
// attempted on Register/Deregister and failures surface through the retry
// loop instead of failing the boot. For the config source use NewEtcdClient
// (with probe): eager-load semantics require fail-fast at startup.
func NewEtcdClientLazy(cfg EtcdConfig) (*clientv3.Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("backends/etcd: endpoints must not be empty")
	}
	timeout := cfg.DialTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DialTimeout: timeout,
	})
}

// NewEtcdClient constructs a clientv3.Client from cfg.
//
// clientv3.New uses lazy dialing: the constructor itself almost never fails.
// To detect an unreachable cluster at startup, NewEtcdClient probes the first
// endpoint with Status using a context bounded by cfg.DialTimeout (or 5 s if
// zero).  If the probe fails the client is closed and the error is returned.
func NewEtcdClient(cfg EtcdConfig) (*clientv3.Client, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("backends/etcd: endpoints must not be empty")
	}

	timeout := cfg.DialTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DialTimeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("backends/etcd: create client: %w", err)
	}

	// Probe first endpoint to surface connectivity failures early.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if _, err := client.Status(ctx, cfg.Endpoints[0]); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("backends/etcd: probe %q: %w", cfg.Endpoints[0], err)
	}

	return client, nil
}

// ---------------------------------------------------------------------------
// Nacos
// ---------------------------------------------------------------------------

// NacosConfig carries connection parameters for a Nacos server.
// ServerAddrs is a list of "host:port" strings.
type NacosConfig struct {
	ServerAddrs []string `json:"server_addrs" yaml:"server_addrs"`
	Namespace   string   `json:"namespace"    yaml:"namespace"`
	Group       string   `json:"group"        yaml:"group"`
	DataID      string   `json:"data_id"      yaml:"data_id"`
	Username    string   `json:"username"     yaml:"username"`
	Password    string   `json:"password"     yaml:"password"`
	TimeoutMs   uint64   `json:"timeout_ms"   yaml:"timeout_ms"`
}

// NewNacosConfigClient builds a nacos-sdk-go v1 IConfigClient from cfg.
//
// The nacos SDK self-builds its connection on first use; this constructor only
// validates configuration and instantiates the client object.  Unreachable
// servers will surface on the first Watch/Get call (i.e., config.Load()).
func NewNacosConfigClient(cfg NacosConfig) (config_client.IConfigClient, error) {
	if len(cfg.ServerAddrs) == 0 {
		return nil, fmt.Errorf("backends/nacos: server_addrs must not be empty")
	}

	timeoutMs := cfg.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 10_000 // nacos SDK default
	}

	serverConfigs := make([]constant.ServerConfig, 0, len(cfg.ServerAddrs))
	for _, addr := range cfg.ServerAddrs {
		u, err := url.Parse("nacos://" + addr) // cheap host:port parse
		if err != nil {
			return nil, fmt.Errorf("backends/nacos: parse server addr %q: %w", addr, err)
		}
		port := uint64(8848) // nacos default port
		if u.Port() != "" {
			var p uint64
			if _, err := fmt.Sscanf(u.Port(), "%d", &p); err == nil {
				port = p
			}
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: u.Hostname(),
			Port:   port,
		})
	}

	clientCfg := constant.NewClientConfig(
		constant.WithNamespaceId(cfg.Namespace),
		constant.WithTimeoutMs(timeoutMs),
		constant.WithUsername(cfg.Username),
		constant.WithPassword(cfg.Password),
		constant.WithNotLoadCacheAtStart(true),
	)

	c, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  clientCfg,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("backends/nacos: create config client: %w", err)
	}

	return c, nil
}

// NewNacosNamingClient builds a nacos-sdk-go v2 INamingClient from cfg.
//
// The nacos SDK v2 builds its connection on first use; this constructor only
// validates configuration and instantiates the client object.  Unreachable
// servers will surface on the first Register/GetService call.
//
// Note: nacos-sdk-go v1 (IConfigClient) and v2 (INamingClient) coexist in the
// same module; they use different import paths and are wholly independent.
func NewNacosNamingClient(cfg NacosConfig) (naming_client.INamingClient, error) {
	if len(cfg.ServerAddrs) == 0 {
		return nil, fmt.Errorf("backends/nacos: server_addrs must not be empty")
	}

	timeoutMs := cfg.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 10_000
	}

	serverConfigs := make([]constantv2.ServerConfig, 0, len(cfg.ServerAddrs))
	for _, addr := range cfg.ServerAddrs {
		u, err := url.Parse("nacos://" + addr)
		if err != nil {
			return nil, fmt.Errorf("backends/nacos: parse server addr %q: %w", addr, err)
		}
		port := uint64(8848)
		if u.Port() != "" {
			var p uint64
			if _, err := fmt.Sscanf(u.Port(), "%d", &p); err == nil {
				port = p
			}
		}
		serverConfigs = append(serverConfigs, *constantv2.NewServerConfig(u.Hostname(), port))
	}

	clientCfg := constantv2.NewClientConfig(
		constantv2.WithNamespaceId(cfg.Namespace),
		constantv2.WithTimeoutMs(timeoutMs),
		constantv2.WithUsername(cfg.Username),
		constantv2.WithPassword(cfg.Password),
		constantv2.WithNotLoadCacheAtStart(true),
	)

	c, err := clientsv2.NewNamingClient(vov2.NacosClientParam{
		ClientConfig:  clientCfg,
		ServerConfigs: serverConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("backends/nacos: create naming client: %w", err)
	}

	return c, nil
}

// ---------------------------------------------------------------------------
// Kubernetes
// ---------------------------------------------------------------------------

// K8sConfig carries parameters for the Kubernetes ConfigMap config source.
//
// The contrib kubernetes source self-builds its REST client from KubeconfigPath
// (empty = in-cluster).  This layer carries the structural config only; no
// client object is constructed here.  That design is intentional: the contrib
// source does not accept an injected client, so the connection layer is
// effectively inside the contrib package itself.
//
// Note: "the backend SDK self-builds the connection; the access layer only
// carries configuration."
//
// Granularity note: the contrib source selects a ConfigMap by name (via a
// metadata.name FieldSelector built from Name) and then loads ALL keys under
// that ConfigMap's data, emitting one config entry per key.  Each entry's
// format is inferred from the key's file extension (e.g. "runtime.yaml" → yaml,
// "app.json" → json).  There is no single-data-key selection: contrib has no
// such option and iterates the whole data map.  Therefore K8sConfig carries no
// data-key field — put exactly the keys you want loaded into the ConfigMap.
type K8sConfig struct {
	// Namespace is the Kubernetes namespace containing the ConfigMap.
	Namespace string `json:"namespace"       yaml:"namespace"`
	// Name is the ConfigMap name.  All keys under the selected ConfigMap's data
	// are loaded; each key's format is inferred from its file extension.
	Name string `json:"name"            yaml:"name"`
	// KubeconfigPath is the path to a kubeconfig file.  Empty means in-cluster.
	KubeconfigPath string `json:"kubeconfig_path" yaml:"kubeconfig_path"`
}
