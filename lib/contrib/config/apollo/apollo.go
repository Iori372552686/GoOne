package apollo

import (
	stdlog "log"
	"strings"

	"github.com/Iori372552686/GoOne/lib/contrib/config"

	"github.com/apolloconfig/agollo/v4"
	apolloConfig "github.com/apolloconfig/agollo/v4/env/config"
)

type apollo struct {
	client agollo.Client
	opt    *options
}

// Option is apollo option
type Option func(*options)

type options struct {
	appid          string
	secret         string
	cluster        string
	endpoint       string
	namespace      string
	isBackupConfig bool
	backupPath     string

	logger Logger
}

// Logger is a tiny logger interface to keep config decoupled from external ecosystems.
type Logger interface {
	Warnf(format string, args ...any)
}

type stdLogger struct{}

func (stdLogger) Warnf(format string, args ...any) {
	stdlog.Printf("[apollo-config] "+format, args...)
}

// WithAppID with apollo config app id
func WithAppID(appID string) Option {
	return func(o *options) {
		o.appid = appID
	}
}

// WithCluster with apollo config cluster
func WithCluster(cluster string) Option {
	return func(o *options) {
		o.cluster = cluster
	}
}

// WithEndpoint with apollo config config server ip
func WithEndpoint(endpoint string) Option {
	return func(o *options) {
		o.endpoint = endpoint
	}
}

// WithEnableBackup with apollo config enable backup config
func WithEnableBackup() Option {
	return func(o *options) {
		o.isBackupConfig = true
	}
}

// WithDisableBackup with apollo config enable backup config
func WithDisableBackup() Option {
	return func(o *options) {
		o.isBackupConfig = false
	}
}

// WithSecret with apollo config app secret
func WithSecret(secret string) Option {
	return func(o *options) {
		o.secret = secret
	}
}

// WithNamespace with apollo config namespace name
func WithNamespace(name string) Option {
	return func(o *options) {
		o.namespace = name
	}
}

// WithBackupPath with apollo config backupPath
func WithBackupPath(backupPath string) Option {
	return func(o *options) {
		o.backupPath = backupPath
	}
}

// WithLogger use custom logger to replace default logger.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// NewSource Deprecated: P1-07 起推荐使用 MustNewSource（语义更明确）或 NewSourceE
// （error 返回，库/工厂优先）。本函数保留一个版本以兼容旧调用方，内部委托 MustNewSource。
func NewSource(opts ...Option) config.Source {
	return MustNewSource(opts...)
}

// MustNewSource 构造 Apollo 配置源，失败时 panic（P1-07）。
// 适用于「配置错误即程序错误」的启动期装配；库/工厂代码请用 NewSourceE。
func MustNewSource(opts ...Option) config.Source {
	src, err := NewSourceE(opts...)
	if err != nil {
		panic(err)
	}
	return src
}

// NewSourceE is an error-returning variant of NewSource.
// It is preferred for library/factory usage to avoid panics.
func NewSourceE(opts ...Option) (config.Source, error) {
	op := options{
		logger: stdLogger{},
	}
	for _, o := range opts {
		o(&op)
	}
	client, err := agollo.StartWithConfig(func() (*apolloConfig.AppConfig, error) {
		return &apolloConfig.AppConfig{
			AppID:            op.appid,
			Cluster:          op.cluster,
			NamespaceName:    op.namespace,
			IP:               op.endpoint,
			IsBackupConfig:   op.isBackupConfig,
			Secret:           op.secret,
			BackupConfigPath: op.backupPath,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &apollo{client: client, opt: &op}, nil
}

// genKey got the key of config.KeyValue pair.
// eg: namespace.ext with subKey got namespace.subKey
func genKey(ns, sub string) string {
	arr := strings.Split(ns, ".")
	if len(arr) < 1 {
		return sub
	}

	if len(arr) == 1 {
		if ns == "" {
			return sub
		}
		return ns + "." + sub
	}

	return strings.Join(arr[:len(arr)-1], ".") + "." + sub
}

// resolve convert kv pair into one map[string]interface{} by split key into different
// map level. such as: app.name = "application" => map[app][name] = "application"
func resolve(key string, value interface{}, target map[string]interface{}) {
	// expand key "aaa.bbb" into map[aaa]map[bbb]interface{}
	keys := strings.Split(key, ".")
	last := len(keys) - 1
	cursor := target

	for i, k := range keys {
		if i == last {
			cursor[k] = value
			break
		}

		// not the last key, be deeper
		v, ok := cursor[k]
		if !ok {
			// create a new map
			deeper := make(map[string]interface{})
			cursor[k] = deeper
			cursor = deeper
			continue
		}

		// current exists, then check existing value type, if it's not map
		// that means duplicate keys, and at least one is not map instance.
		if cursor, ok = v.(map[string]interface{}); !ok {
			stdlog.Printf("[apollo-config] duplicate key: %v", strings.Join(keys[:i+1], "."))
			break
		}
	}
}

func format(ns string) string {
	arr := strings.Split(ns, ".")
	if len(arr) <= 1 {
		return "json"
	}

	return arr[len(arr)-1]
}

func (e *apollo) load() []*config.KeyValue {
	kv := make([]*config.KeyValue, 0)
	namespaces := strings.Split(e.opt.namespace, ",")

	for _, ns := range namespaces {
		next := map[string]interface{}{}
		e.client.GetConfigCache(ns).Range(func(key, value interface{}) bool {
			// all values are out properties format
			resolve(genKey(ns, key.(string)), value, next)
			return true
		})

		// serialize the namespace content KeyValue into bytes.
		f := format(ns)
		val, err := config.Marshal(f, next)
		if err != nil {
			if e.opt != nil && e.opt.logger != nil {
				e.opt.logger.Warnf("apollo could not handle namespace %s: %v", ns, err)
			}
			continue
		}

		kv = append(kv, &config.KeyValue{
			Key:    ns,
			Value:  val,
			Format: f,
		})
	}

	return kv
}

func (e *apollo) Load() (kv []*config.KeyValue, err error) {
	return e.load(), nil
}

func (e *apollo) Watch() (config.Watcher, error) {
	w, err := newWatcher(e, e.opt.logger)
	if err != nil {
		return nil, err
	}
	return w, nil
}
