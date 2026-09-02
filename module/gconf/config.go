// Package gconf 定义 GoOne 各服务的配置 struct 类型。
//
// 历史上本包还承担加载、全局变量存储、normalize/validate 三项职责，存在 6 倍复制
// 粘贴（6 个 XxxConfig struct + 6 个 LoadXxxConfig + 6 套 normalize/validate）与包级
// 可变全局。这些已在配置体系重构中迁出：
//
//   - 加载/存储/key 访问：module/conf（多格式 decoder + 不可变快照 + Get/Unmarshal）。
//   - normalize/validate：module/conf/registry.go（Registrar 注册式，消除重复）。
//   - admin 端口回退：conf.ResolveAdminPort（魔法 int 改命名常量）。
//
// 本包现仅保留 struct 类型定义，作为业务代码 conf.Unmarshal 的目标类型（它们有 yaml
// tag 与字段文档，是有价值的字段契约）。配置读取统一走 conf：
//
//	var cap gconf.ConnCapacityConfig
//	conf.Unmarshal("connsvr.capacity", &cap)
package gconf

import (
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	gormdb "github.com/Iori372552686/GoOne/lib/db/gorm"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
)

type RuntimeTracingConfig struct {
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Exporter     string            `json:"exporter" yaml:"exporter"`
	Endpoint     string            `json:"endpoint" yaml:"endpoint"`
	Insecure     bool              `json:"insecure" yaml:"insecure"`
	SamplerRatio float64           `json:"sampler_ratio" yaml:"sampler_ratio"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
}

type BaseRuntimeConfig struct {
	// ParseConfig parses registry address strings like:
	//   - "127.0.0.1:2181"                       (defaults to zk)
	//   - "zk://127.0.0.1:2181?root=/&service=online&timeout=30s"
	//   - "etcd://127.0.0.1:2379,127.0.0.2:2379?namespace=/microservices&service=online&timeout=5s"
	//   - "consul://127.0.0.1:8500?service=online&healthcheck=true&heartbeat=true&health_interval=10"
	//   - "nacos://127.0.0.1:8848?service=online&group=DEFAULT_GROUP&cluster=DEFAULT&kind=grpc&weight=100"
	//   - "k8s://?service=online&incluster=true"
	RegisterAddr string               `json:"register_addr" yaml:"register_addr"` // registry/register 地址
	BusMQAddr    string               `json:"bus_mq_addr" yaml:"bus_mq_addr"`     // bus mq 地址
	AdminServer  AdminServerConfig    `json:"admin_server" yaml:"admin_server"`   // admin server 配置
	Tracing      RuntimeTracingConfig `json:"tracing" yaml:"tracing"`
}

type BaseDependenciesConfig struct {
	GameDataDir        string             `json:"game_data_dir" yaml:"game_data_dir"`               // 游戏数据目录
	SensitiveWordsFile string             `json:"sensitive_words_file" yaml:"sensitive_words_file"` // 敏感词文件
	NacosConf          net_conf.NacosConf `json:"nacos_conf" yaml:"nacos_conf"`                     // nacos配置
	OrmConf            []gormdb.Config    `json:"orm_instances" yaml:"orm_instances"`               // mysql配置
	HTTPSigns          []http_sign.Config `json:"http_sign" yaml:"http_sign"`                       // http签名配置
	RestApiConf        []rest_api.Config  `json:"rest_api_config" yaml:"rest_api_config"`           // restapi配置
	DbInstances        []redis.Config     `json:"db_instances" yaml:"db_instances"`                 // redis配置
}

type BaseDebugConfig struct {
	Pprof bool `json:"pprof" yaml:"pprof"` // 是否开启pprof
}

type BaseCfg struct {
	CommonRuntime BaseRuntimeConfig      `json:"runtime" yaml:"runtime"`
	Dependencies  BaseDependenciesConfig `json:"dependencies" yaml:"dependencies"`
	CommonDebug   BaseDebugConfig        `json:"debug" yaml:"debug"`
}

type AdminServerConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"` // 是否开启统一 admin server
	IP      string `json:"ip" yaml:"ip"`           // 监听 ip，为空时监听全部网卡
	Port    int    `json:"port" yaml:"port"`       // 监听端口，为 0 时按服务类型回退到默认端口
}

type ServiceIdentityConfig struct {
	SelfBusId string `json:"self_bus_id" yaml:"self_bus_id"`
}

type ServiceDebugConfig struct {
	LogDir   string `json:"log_dir" yaml:"log_dir"`
	LogLevel string `json:"log_level" yaml:"log_level"`
}

type ServiceCommonConfig struct {
	Identity ServiceIdentityConfig `json:"identity" yaml:"identity"`
	Debug    ServiceDebugConfig    `json:"debug" yaml:"debug"`
}

type ConnRuntimeConfig struct {
	ListenPort int `json:"listen_port" yaml:"listen_port"`
	// TcpImplType 选择 TCP 后端："gonet"/空 = 每连接 goroutine（默认），
	// "gnet" = epoll/kqueue 事件驱动（万级连接场景）。
	TcpImplType string `json:"tcp_impl_type" yaml:"tcp_impl_type"`
	// KcpPort > 0 时额外启动 KCP(UDP) 网关，供弱网/实时性敏感客户端使用。
	KcpPort int `json:"kcp_port" yaml:"kcp_port"`
}

// ConnCapacityConfig 定义 connsvr 网关的过载保护参数。
// 所有字段 0 值表示"不限制"（向后兼容）；生产配置应显式设置。
// OverloadMode 控制策略：off（不限）、shadow（只统计不拒绝）、enforce（拒绝）。
type ConnCapacityConfig struct {
	// MaxConnections 总连接数上限（含未认证）。0=不限。
	MaxConnections int64 `json:"max_connections" yaml:"max_connections"`
	// MaxUnauthenticatedConnections 未认证连接上限（已连接但未 BindClient）。
	MaxUnauthenticatedConnections int64 `json:"max_unauthenticated_connections" yaml:"max_unauthenticated_connections"`
	// ConnectionRate 每秒新建连接数上限。0=不限。
	ConnectionRate int `json:"connection_rate" yaml:"connection_rate"`
	// LoginRate 每秒首次登录（BindClient）数上限。0=不限。
	LoginRate int `json:"login_rate" yaml:"login_rate"`
	// MaxInflight SSRPC 全局并发在途请求数上限。0=不限。
	MaxInflight int `json:"max_inflight" yaml:"max_inflight"`
	// MaxInflightPerMethod 按 method 名覆盖 MaxInflight。key 为 method 名。
	MaxInflightPerMethod map[string]int `json:"max_inflight_per_method" yaml:"max_inflight_per_method"`
	// OverloadMode 取值 off/shadow/enforce。shadow 只统计上报不拒绝；enforce 超限拒绝。
	OverloadMode string `json:"overload_mode" yaml:"overload_mode"`
}

type MainCapacityConfig struct {
	RoleSyncPatchEnabled   bool     `json:"role_sync_patch_enabled" yaml:"role_sync_patch_enabled"`
	RoleSyncPatchAllowUids []uint64 `json:"role_sync_patch_allow_uids" yaml:"role_sync_patch_allow_uids"`
	RolePersistDebounceSec int      `json:"role_persist_debounce_sec" yaml:"role_persist_debounce_sec"`
}

type WebRuntimeConfig struct {
	HttpServer web_gin.Config   `json:"http_server" yaml:"http_server"`
	GRPCServer GRPCServerConfig `json:"grpc_server" yaml:"grpc_server"`
}

type ConnSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Runtime             ConnRuntimeConfig  `json:"runtime" yaml:"runtime"`
	Capacity            ConnCapacityConfig `json:"capacity" yaml:"capacity"`
}

type InfoSvr struct {
	ServiceCommonConfig `yaml:",inline"`
}

type MainSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Capacity            MainCapacityConfig `json:"capacity" yaml:"capacity"`
}

type MySqlSvr struct {
	ServiceCommonConfig `yaml:",inline"`
}

type RoomCenterSvr struct {
	ServiceCommonConfig `yaml:",inline"`
}

type WebSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Runtime             WebRuntimeConfig `json:"runtime" yaml:"runtime"`
}

type GRPCServerConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	IP      string `json:"ip" yaml:"ip"`
	Port    int    `json:"port" yaml:"port"`
	// Reflection 仅在显式开启时注册 gRPC reflection 服务，避免生产环境暴露
	// 服务元数据。默认 false。
	Reflection bool `json:"reflection" yaml:"reflection"`
}
