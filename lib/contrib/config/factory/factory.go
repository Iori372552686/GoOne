package factory

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	contribconfig "github.com/Iori372552686/GoOne/lib/contrib/config"
	conf_apollo "github.com/Iori372552686/GoOne/lib/contrib/config/apollo"
	conf_consul "github.com/Iori372552686/GoOne/lib/contrib/config/consul"
	conf_k8s "github.com/Iori372552686/GoOne/lib/contrib/config/kubernetes"
	conf_nacos "github.com/Iori372552686/GoOne/lib/contrib/config/nacos"

	"github.com/hashicorp/consul/api"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

type Backend string

const (
	BackendConsul Backend = "consul"
	BackendEtcd   Backend = "etcd"
	BackendK8S    Backend = "k8s"
	BackendApollo Backend = "apollo"
	BackendNacos  Backend = "nacos"
)

// Config is a string-friendly configuration to build a config Source.
type Config struct {
	Backend Backend
	Addrs   []string
	Timeout time.Duration

	// Common: path in config center.
	Path string

	// etcd options.
	EtcdPrefix   bool
	EtcdUserName string
	EtcdPassword string

	// Kubernetes options.
	KubeNamespace    string
	KubeLabelSelect  string
	KubeFieldSelect  string
	KubeConfig       string
	KubeMaster       string

	// Apollo options.
	ApolloAppID      string
	ApolloSecret     string
	ApolloCluster    string
	ApolloEndpoint   string
	ApolloNamespaces string // comma-separated
	ApolloBackup     bool
	ApolloBackupPath string

	// Nacos options.
	NacosDataIDs      []string
	NacosGroup        string
	NacosNamespaceID  string
	NacosUserName     string
	NacosPassword     string
	NacosLogDir       string
	NacosCacheDir     string
	NacosLogLevel     string
}

func (c *Config) Normalize() error {
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	switch c.Backend {
	case BackendConsul:
		if len(c.Addrs) == 0 {
			return fmt.Errorf("consul: missing address")
		}
		if strings.TrimSpace(c.Path) == "" {
			return fmt.Errorf("consul: missing path")
		}
	case BackendEtcd:
		if len(c.Addrs) == 0 {
			return fmt.Errorf("etcd: missing address")
		}
		if strings.TrimSpace(c.Path) == "" {
			return fmt.Errorf("etcd: missing path")
		}
	case BackendK8S:
		if strings.TrimSpace(c.KubeNamespace) == "" {
			return fmt.Errorf("k8s: missing namespace")
		}
	case BackendApollo:
		if strings.TrimSpace(c.ApolloAppID) == "" {
			return fmt.Errorf("apollo: missing appid")
		}
		if strings.TrimSpace(c.ApolloEndpoint) == "" {
			return fmt.Errorf("apollo: missing endpoint")
		}
		if strings.TrimSpace(c.ApolloNamespaces) == "" {
			return fmt.Errorf("apollo: missing namespace(s)")
		}
	case BackendNacos:
		if len(c.Addrs) == 0 {
			return fmt.Errorf("nacos: missing address")
		}
		if len(c.NacosDataIDs) == 0 {
			return fmt.Errorf("nacos: missing dataid(s)")
		}
	default:
		return fmt.Errorf("unsupported config backend: %q", c.Backend)
	}
	return nil
}

// ParseConfig parses a single address string into a Config.
//
// Examples:
// - consul://127.0.0.1:8500?path=goone/config
// - etcd://127.0.0.1:2379?path=/goone/config&prefix=true&timeout=5s
// - k8s://?namespace=default&label=app=goone&kubeconfig=/path/to/kubeconfig
// - apollo://?appid=goone&endpoint=http://127.0.0.1:8080&cluster=dev&namespace=application.yaml,demo.json&backup=true
// - nacos://127.0.0.1:8848?dataid=app.yaml,db.yaml&group=DEFAULT_GROUP&namespace_id=public&timeout=5s
func ParseConfig(addr string) (Config, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return Config{}, fmt.Errorf("config addr is empty")
	}
	if !strings.Contains(addr, "://") {
		return Config{}, fmt.Errorf("config addr must include scheme (consul://, etcd://, k8s://, apollo://): %q", addr)
	}
	u, err := url.Parse(addr)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{Timeout: 30 * time.Second}
	switch strings.ToLower(u.Scheme) {
	case "consul":
		cfg.Backend = BackendConsul
	case "etcd":
		cfg.Backend = BackendEtcd
	case "k8s", "kubernetes":
		cfg.Backend = BackendK8S
	case "apollo":
		cfg.Backend = BackendApollo
	case "nacos":
		cfg.Backend = BackendNacos
	default:
		return Config{}, fmt.Errorf("unsupported config scheme: %q", u.Scheme)
	}

	// allow comma-separated hosts in host part
	hostPart := u.Host
	if hostPart == "" && strings.HasPrefix(u.Path, "/") && strings.Contains(u.Path, ":") {
		// tolerate "etcd:/127.0.0.1:2379"
		hostPart = strings.TrimPrefix(u.Path, "/")
		u.Path = ""
	}
	cfg.Addrs = splitAddrs(hostPart)

	q := u.Query()
	if v := strings.TrimSpace(q.Get("timeout")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid timeout: %q: %w", v, err)
		}
		cfg.Timeout = d
	}
	if v := strings.TrimSpace(q.Get("path")); v != "" {
		cfg.Path = v
	}
	if v := strings.TrimSpace(q.Get("prefix")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid prefix: %q: %w", v, err)
		}
		cfg.EtcdPrefix = b
	}
	// etcd 鉴权（可选）；公开 etcd 常带账号密码，与 username/password query 对齐。
	if v := strings.TrimSpace(q.Get("username")); v != "" {
		cfg.EtcdUserName = v
	}
	if v := strings.TrimSpace(q.Get("password")); v != "" {
		cfg.EtcdPassword = v
	}

	// k8s params
	if v := strings.TrimSpace(q.Get("namespace")); v != "" {
		cfg.KubeNamespace = v
	}
	if v := strings.TrimSpace(q.Get("label")); v != "" {
		cfg.KubeLabelSelect = v
	}
	if v := strings.TrimSpace(q.Get("field")); v != "" {
		cfg.KubeFieldSelect = v
	}
	if v := strings.TrimSpace(q.Get("kubeconfig")); v != "" {
		cfg.KubeConfig = v
	}
	if v := strings.TrimSpace(q.Get("master")); v != "" {
		cfg.KubeMaster = v
	}

	// apollo params
	if v := strings.TrimSpace(q.Get("appid")); v != "" {
		cfg.ApolloAppID = v
	}
	if v := strings.TrimSpace(q.Get("secret")); v != "" {
		cfg.ApolloSecret = v
	}
	if v := strings.TrimSpace(q.Get("cluster")); v != "" {
		cfg.ApolloCluster = v
	}
	if v := strings.TrimSpace(q.Get("endpoint")); v != "" {
		cfg.ApolloEndpoint = v
	}
	if v := strings.TrimSpace(q.Get("namespace")); v != "" {
		cfg.ApolloNamespaces = v
	}
	if v := strings.TrimSpace(q.Get("backup")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid backup: %q: %w", v, err)
		}
		cfg.ApolloBackup = b
	}
	if v := strings.TrimSpace(q.Get("backup_path")); v != "" {
		cfg.ApolloBackupPath = v
	}

	// nacos params
	if v := strings.TrimSpace(q.Get("dataid")); v != "" {
		cfg.NacosDataIDs = splitAddrs(v)
	}
	if v := strings.TrimSpace(q.Get("group")); v != "" {
		cfg.NacosGroup = v
	}
	if v := strings.TrimSpace(q.Get("namespace_id")); v != "" {
		cfg.NacosNamespaceID = v
	}
	if v := strings.TrimSpace(q.Get("username")); v != "" {
		cfg.NacosUserName = v
	}
	if v := strings.TrimSpace(q.Get("password")); v != "" {
		cfg.NacosPassword = v
	}
	if v := strings.TrimSpace(q.Get("logdir")); v != "" {
		cfg.NacosLogDir = v
	}
	if v := strings.TrimSpace(q.Get("cachedir")); v != "" {
		cfg.NacosCacheDir = v
	}
	if v := strings.TrimSpace(q.Get("loglevel")); v != "" {
		cfg.NacosLogLevel = v
	}

	if err := cfg.Normalize(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// NewClient creates a config client from Config.
func NewClient(cfg Config) (contribconfig.Client, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	switch cfg.Backend {
	case BackendConsul:
		cc := api.DefaultConfig()
		cc.Address = cfg.Addrs[0]
		cli, err := api.NewClient(cc)
		if err != nil {
			return nil, err
		}
		src, err := conf_consul.New(cli,
			conf_consul.WithContext(context.Background()),
			conf_consul.WithPath(cfg.Path),
		)
		if err != nil {
			return nil, err
		}
		return contribconfig.Wrap(src, nil), nil

	case BackendK8S:
		src := conf_k8s.NewSource(
			conf_k8s.Namespace(cfg.KubeNamespace),
			conf_k8s.LabelSelector(cfg.KubeLabelSelect),
			conf_k8s.FieldSelector(cfg.KubeFieldSelect),
			conf_k8s.KubeConfig(cfg.KubeConfig),
			conf_k8s.Master(cfg.KubeMaster),
		)
		return contribconfig.Wrap(src, nil), nil

	case BackendApollo:
		opts := []conf_apollo.Option{
			conf_apollo.WithAppID(cfg.ApolloAppID),
			conf_apollo.WithEndpoint(cfg.ApolloEndpoint),
			conf_apollo.WithNamespace(cfg.ApolloNamespaces),
		}
		if cfg.ApolloCluster != "" {
			opts = append(opts, conf_apollo.WithCluster(cfg.ApolloCluster))
		}
		if cfg.ApolloSecret != "" {
			opts = append(opts, conf_apollo.WithSecret(cfg.ApolloSecret))
		}
		if cfg.ApolloBackup {
			opts = append(opts, conf_apollo.WithEnableBackup())
		} else {
			opts = append(opts, conf_apollo.WithDisableBackup())
		}
		if cfg.ApolloBackupPath != "" {
			opts = append(opts, conf_apollo.WithBackupPath(cfg.ApolloBackupPath))
		}

		src, err := conf_apollo.NewSourceE(opts...)
		if err != nil {
			return nil, err
		}
		return contribconfig.Wrap(src, nil), nil

	case BackendNacos:
		nc, err := newNacosConfigClient(cfg)
		if err != nil {
			return nil, err
		}
		group := cfg.NacosGroup
		if strings.TrimSpace(group) == "" {
			group = "DEFAULT_GROUP"
		}
		src, err := conf_nacos.New(nc,
			conf_nacos.WithContext(context.Background()),
			conf_nacos.WithGroup(group),
			conf_nacos.WithDataIDs(cfg.NacosDataIDs...),
		)
		if err != nil {
			return nil, err
		}
		// nacos config client currently doesn't expose a reliable Close; use no-op.
		return contribconfig.Wrap(src, nil), nil

	case BackendEtcd:
		return newEtcdClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported config backend: %q", cfg.Backend)
	}
}

// newNacosConfigClient 构造一个 nacos config client（读/写共用）。
// 从 NewClient 的 nacos 分支提取，行为零变化；NewPublisher 也复用此构造。
func newNacosConfigClient(cfg Config) (config_client.IConfigClient, error) {
	// server conf
	sc := make([]constant.ServerConfig, 0, len(cfg.Addrs))
	for _, a := range cfg.Addrs {
		host, port, err := splitHostPortDefault(a, 8848)
		if err != nil {
			return nil, fmt.Errorf("nacos: invalid addr %q: %w", a, err)
		}
		sc = append(sc, *constant.NewServerConfig(host, uint64(port)))
	}
	// client conf
	cc := constant.ClientConfig{
		TimeoutMs:           uint64(maxInt64(1000, cfg.Timeout.Milliseconds())),
		NotLoadCacheAtStart: true,
		NamespaceId:         cfg.NacosNamespaceID,
		LogDir:              cfg.NacosLogDir,
		CacheDir:            cfg.NacosCacheDir,
		LogLevel:            cfg.NacosLogLevel,
		Username:            cfg.NacosUserName,
		Password:            cfg.NacosPassword,
	}
	return clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	})
}

func splitHostPortDefault(s string, defaultPort int) (string, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	if strings.Contains(s, ":") {
		u, err := url.Parse("dummy://" + s)
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
	return s, defaultPort, nil
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func NewFromAddr(addr string) (contribconfig.Client, Config, error) {
	cfg, err := ParseConfig(addr)
	if err != nil {
		return nil, Config{}, err
	}
	c, err := NewClient(cfg)
	if err != nil {
		return nil, Config{}, err
	}
	return c, cfg, nil
}

// NewPublisher 按 Config 构造一个配置中心写 Publisher，风格与 NewClient 对称：
// 按 cfg.Backend 切换到对应后端实现。读（Source）与写（Publisher）共用同一套
// 地址/鉴权解析（ParseConfig），调用方只需提供同一个 URL。
func NewPublisher(cfg Config) (contribconfig.Publisher, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	switch cfg.Backend {
	case BackendNacos:
		nc, err := newNacosConfigClient(cfg)
		if err != nil {
			return nil, err
		}
		group := cfg.NacosGroup
		if strings.TrimSpace(group) == "" {
			group = "DEFAULT_GROUP"
		}
		return conf_nacos.NewPublisher(nc, conf_nacos.WithPubGroup(group))

	case BackendEtcd:
		return newEtcdPublisher(cfg)

	default:
		return nil, fmt.Errorf("publisher not implemented for backend: %q (only nacos/etcd)", cfg.Backend)
	}
}

// NewPublisherFromURL 是便捷入口：解析 URL 后构造 Publisher（与 NewFromAddr 对称）。
func NewPublisherFromURL(addr string) (contribconfig.Publisher, Config, error) {
	cfg, err := ParseConfig(addr)
	if err != nil {
		return nil, Config{}, err
	}
	p, err := NewPublisher(cfg)
	if err != nil {
		return nil, Config{}, err
	}
	return p, cfg, nil
}

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


