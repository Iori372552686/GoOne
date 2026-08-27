package factory

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/lib/contrib/registry"
	reg_consul "github.com/Iori372552686/GoOne/lib/contrib/registry/consul"
	reg_k8s "github.com/Iori372552686/GoOne/lib/contrib/registry/kubernetes"
	reg_nacos "github.com/Iori372552686/GoOne/lib/contrib/registry/nacos"
	reg_zk "github.com/Iori372552686/GoOne/lib/contrib/registry/zookeeper"

	"github.com/hashicorp/consul/api"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Backend string

const (
	BackendZooKeeper Backend = "zk"
	BackendEtcd      Backend = "etcd"
	BackendConsul    Backend = "consul"
	BackendK8S       Backend = "k8s"
	BackendNacos     Backend = "nacos"
)

// Config describes how to create a registry client and how to use it.
// It is intentionally small & string-based for easy CLI/config usage.
type Config struct {
	Backend     Backend
	Addrs       []string
	Timeout     time.Duration
	ServiceName string

	// ZooKeeper options.
	RootPath string

	// etcd options.
	Namespace string

	// Consul options.
	ConsulHealthCheck         bool
	ConsulHeartbeat           bool
	ConsulHealthCheckInterval int

	// Nacos options.
	NacosGroup       string
	NacosCluster     string
	NacosKind        string
	NacosWeight      float64
	NacosNamespaceID string
	NacosUserName    string
	NacosPassword    string

	// Kubernetes options.
	KubeConfig string
	InCluster  bool
}

// Normalize fills backend-specific defaults and validates basic fields.
func (c *Config) Normalize() error {
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	if strings.TrimSpace(c.ServiceName) == "" {
		c.ServiceName = "online"
	}
	switch c.Backend {
	case BackendZooKeeper:
		if strings.TrimSpace(c.RootPath) == "" {
			c.RootPath = "/"
		}
	case BackendEtcd, BackendConsul, BackendNacos:
		// ok
	case BackendK8S:
		// ok (can run without Addrs)
	default:
		return fmt.Errorf("unsupported registry backend: %q", c.Backend)
	}
	return nil
}

// ParseConfig parses registry address strings like:
//   - "127.0.0.1:2181"                       (defaults to zk)
//   - "zk://127.0.0.1:2181?root=/&service=online&timeout=30s"
//   - "etcd://127.0.0.1:2379,127.0.0.2:2379?namespace=/microservices&service=online&timeout=5s"
//   - "consul://127.0.0.1:8500?service=online&healthcheck=true&heartbeat=true&health_interval=10"
//   - "nacos://127.0.0.1:8848?service=online&group=DEFAULT_GROUP&cluster=DEFAULT&kind=grpc&weight=100"
//   - "k8s://?service=online&incluster=true"
func ParseConfig(addr string) (Config, error) {
	cfg := Config{
		Backend:     BackendZooKeeper,
		Addrs:       nil,
		Timeout:     30 * time.Second,
		ServiceName: "online",
		RootPath:    "/",
		Namespace:   "",

		ConsulHealthCheck:         true,
		ConsulHeartbeat:           true,
		ConsulHealthCheckInterval: 10,

		NacosGroup:       "DEFAULT_GROUP",
		NacosCluster:     "DEFAULT",
		NacosKind:        "grpc",
		NacosWeight:      100,
		NacosNamespaceID: "",

		KubeConfig: "",
		InCluster:  false,
	}

	addr = strings.TrimSpace(addr)
	if addr == "" {
		return cfg, fmt.Errorf("registry addr is empty")
	}

	// No scheme -> default ZooKeeper (backward compatible).
	if !strings.Contains(addr, "://") {
		cfg.Addrs = splitAddrs(addr)
		if len(cfg.Addrs) == 0 {
			return cfg, fmt.Errorf("invalid registry addr: %q", addr)
		}
		return cfg, nil
	}

	u, err := url.Parse(addr)
	if err != nil {
		return cfg, err
	}

	switch strings.ToLower(u.Scheme) {
	case "zk", "zookeeper":
		cfg.Backend = BackendZooKeeper
	case "etcd":
		cfg.Backend = BackendEtcd
	case "consul":
		cfg.Backend = BackendConsul
	case "k8s", "kubernetes":
		cfg.Backend = BackendK8S
	case "nacos":
		cfg.Backend = BackendNacos
	default:
		return cfg, fmt.Errorf("unsupported registry scheme: %q", u.Scheme)
	}

	// hosts: allow comma-separated
	hostPart := u.Host
	// If someone writes zk:/127.0.0.1:2181, url.Parse may put it into Path.
	if hostPart == "" && strings.HasPrefix(u.Path, "/") && strings.Contains(u.Path, ":") {
		hostPart = strings.TrimPrefix(u.Path, "/")
		u.Path = ""
	}
	cfg.Addrs = splitAddrs(hostPart)
	// k8s allows empty hosts.
	if cfg.Backend != BackendK8S && len(cfg.Addrs) == 0 {
		return cfg, fmt.Errorf("missing registry hosts in: %q", addr)
	}

	q := u.Query()
	if v := strings.TrimSpace(q.Get("service")); v != "" {
		cfg.ServiceName = v
	}
	if v := strings.TrimSpace(q.Get("timeout")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid timeout: %q: %w", v, err)
		}
		cfg.Timeout = d
	}
	if v := strings.TrimSpace(q.Get("root")); v != "" {
		cfg.RootPath = v
	}
	if v := strings.TrimSpace(q.Get("namespace")); v != "" {
		cfg.Namespace = v
	}

	// Consul query params.
	if v := strings.TrimSpace(q.Get("healthcheck")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid healthcheck: %q: %w", v, err)
		}
		cfg.ConsulHealthCheck = b
	}
	if v := strings.TrimSpace(q.Get("heartbeat")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid heartbeat: %q: %w", v, err)
		}
		cfg.ConsulHeartbeat = b
	}
	if v := strings.TrimSpace(q.Get("health_interval")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid health_interval: %q: %w", v, err)
		}
		cfg.ConsulHealthCheckInterval = n
	}

	// Nacos query params.
	if v := strings.TrimSpace(q.Get("group")); v != "" {
		cfg.NacosGroup = v
	}
	if v := strings.TrimSpace(q.Get("cluster")); v != "" {
		cfg.NacosCluster = v
	}
	if v := strings.TrimSpace(q.Get("kind")); v != "" {
		cfg.NacosKind = v
	}
	if v := strings.TrimSpace(q.Get("weight")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("invalid weight: %q: %w", v, err)
		}
		cfg.NacosWeight = f
	}
	if v := strings.TrimSpace(q.Get("nacos_namespace")); v != "" {
		cfg.NacosNamespaceID = v
	}
	if v := strings.TrimSpace(q.Get("username")); v != "" {
		cfg.NacosUserName = v
	}
	if v := strings.TrimSpace(q.Get("password")); v != "" {
		cfg.NacosPassword = v
	}

	// Kubernetes query params.
	if v := strings.TrimSpace(q.Get("kubeconfig")); v != "" {
		cfg.KubeConfig = v
	}
	if v := strings.TrimSpace(q.Get("incluster")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid incluster: %q: %w", v, err)
		}
		cfg.InCluster = b
	}

	// Backend-specific defaults.
	if err := cfg.Normalize(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// NewClient creates a registry client from a Config.
func NewClient(cfg Config) (registry.Client, error) {
	switch cfg.Backend {
	case BackendZooKeeper:
		r, err := reg_zk.New(cfg.Addrs,
			reg_zk.WithRootPath(cfg.RootPath),
			reg_zk.WithTimeout(cfg.Timeout),
		)
		if err != nil {
			return nil, err
		}
		return r, nil

	case BackendEtcd:
		return newEtcdClient(cfg)

	case BackendConsul:
		if len(cfg.Addrs) == 0 {
			return nil, fmt.Errorf("consul: missing address")
		}
		cc := api.DefaultConfig()
		cc.Address = cfg.Addrs[0]
		cli, err := api.NewClient(cc)
		if err != nil {
			return nil, err
		}
		r := reg_consul.New(cli,
			reg_consul.WithHealthCheck(cfg.ConsulHealthCheck),
			reg_consul.WithHeartbeat(cfg.ConsulHeartbeat),
			reg_consul.WithHealthCheckInterval(cfg.ConsulHealthCheckInterval),
		)
		return &consulClient{Registry: r}, nil

	case BackendNacos:
		if len(cfg.Addrs) == 0 {
			return nil, fmt.Errorf("nacos: missing server addresses")
		}
		sc := make([]constant.ServerConfig, 0, len(cfg.Addrs))
		for _, a := range cfg.Addrs {
			host, port, err := splitHostPortDefault(a, 8848)
			if err != nil {
				return nil, fmt.Errorf("nacos: invalid addr %q: %w", a, err)
			}
			sc = append(sc, *constant.NewServerConfig(host, uint64(port)))
		}
		cc := constant.ClientConfig{
			NamespaceId: cfg.NacosNamespaceID,
			TimeoutMs:   uint64(maxInt64(1000, cfg.Timeout.Milliseconds())),
			Username:    cfg.NacosUserName,
			Password:    cfg.NacosPassword,
		}
		nc, err := clients.NewNamingClient(vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		})
		if err != nil {
			return nil, err
		}
		r, err := reg_nacos.New(nc,
			reg_nacos.WithGroup(cfg.NacosGroup),
			reg_nacos.WithCluster(cfg.NacosCluster),
			reg_nacos.WithDefaultKind(cfg.NacosKind),
			reg_nacos.WithWeight(cfg.NacosWeight),
		)
		if err != nil {
			return nil, err
		}
		return &nacosClient{Registry: r}, nil

	case BackendK8S:
		restCfg, err := buildKubeRestConfig(cfg)
		if err != nil {
			return nil, err
		}
		cs, err := kubernetes.NewForConfig(restCfg)
		if err != nil {
			return nil, err
		}
		r := reg_k8s.NewRegistry(cs)
		r.Start()
		return &k8sClient{r: r}, nil

	default:
		return nil, fmt.Errorf("unsupported registry backend: %q", cfg.Backend)
	}
}

// NewFromAddr is a convenience: parse + build.
func NewFromAddr(addr string) (registry.Client, Config, error) {
	cfg, err := ParseConfig(addr)
	if err != nil {
		return nil, Config{}, err
	}
	if err := cfg.Normalize(); err != nil {
		return nil, Config{}, err
	}
	c, err := NewClient(cfg)
	if err != nil {
		return nil, Config{}, err
	}
	return c, cfg, nil
}

type consulClient struct{ *reg_consul.Registry }

func (c *consulClient) Close() error {
	if c.Registry != nil {
		c.Registry.Close()
	}
	return nil
}

type nacosClient struct{ *reg_nacos.Registry }

func (n *nacosClient) Close() error { return nil }

type k8sClient struct{ r *reg_k8s.Registry }

func (k *k8sClient) Register(ctx context.Context, service *registry.ServiceInstance) error {
	return k.r.Register(ctx, service)
}
func (k *k8sClient) Deregister(ctx context.Context, service *registry.ServiceInstance) error {
	return k.r.Deregister(ctx, service)
}
func (k *k8sClient) GetService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	_ = ctx
	return k.r.Service(serviceName)
}
func (k *k8sClient) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	w, err := k.r.Watch(serviceName)
	if err != nil {
		return nil, err
	}
	return &ctxWatcher{ctx: ctx, inner: w}, nil
}
func (k *k8sClient) Close() error {
	k.r.Close()
	return nil
}

type ctxWatcher struct {
	ctx   context.Context
	inner registry.Watcher
}

func (w *ctxWatcher) Next() ([]*registry.ServiceInstance, error) {
	// inner.Next can block, so run it in a goroutine to honor ctx cancellation.
	type res struct {
		s []*registry.ServiceInstance
		e error
	}
	ch := make(chan res, 1)
	go func() {
		s, e := w.inner.Next()
		ch <- res{s: s, e: e}
	}()
	select {
	case <-w.ctx.Done():
		_ = w.inner.Stop()
		return nil, w.ctx.Err()
	case r := <-ch:
		return r.s, r.e
	}
}

func (w *ctxWatcher) Stop() error { return w.inner.Stop() }

// ---- helpers ----

func splitAddrs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func buildKubeRestConfig(cfg Config) (*rest.Config, error) {
	if cfg.KubeConfig != "" {
		return clientcmd.BuildConfigFromFlags("", cfg.KubeConfig)
	}
	if cfg.InCluster {
		return rest.InClusterConfig()
	}
	// Try in-cluster first, otherwise require kubeconfig.
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	return nil, fmt.Errorf("k8s: missing kubeconfig and not running in-cluster (set ?kubeconfig=... or ?incluster=true)")
}

func splitHostPortDefault(s string, defaultPort int) (string, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	// IPv6 may contain ":"; rely on url parsing.
	if strings.Contains(s, ":") {
		host, port, err := splitHostPortStrict(s)
		if err != nil {
			return "", 0, err
		}
		return host, port, nil
	}
	return s, defaultPort, nil
}

func splitHostPortStrict(s string) (string, int, error) {
	hostPort := strings.TrimSpace(s)
	u, err := url.Parse("dummy://" + hostPort)
	if err != nil {
		return "", 0, err
	}
	host := u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("missing host")
	}
	p := u.Port()
	if p == "" {
		return "", 0, fmt.Errorf("missing port")
	}
	pi, err := strconv.Atoi(p)
	if err != nil {
		return "", 0, err
	}
	return host, pi, nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}


