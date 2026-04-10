package gconf

import (
	"flag"
	"fmt"
	"reflect"
	"strings"

	"github.com/Iori372552686/GoOne/lib/web/web_gin"

	"github.com/Iori372552686/GoOne/lib/api/http_sign"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/api/rest_api"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
)

var SvrConfFile = flag.String("svr_conf", "../commconf/server_conf.yaml", "app config yaml file")

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
	RegisterAddr string            `json:"register_addr" yaml:"register_addr"` // registry/register 地址
	BusMQAddr    string            `json:"bus_mq_addr" yaml:"bus_mq_addr"`     // bus mq 地址
	AdminServer  AdminServerConfig `json:"admin_server" yaml:"admin_server"`   // admin server 配置
	Tracing      RuntimeTracingConfig `json:"tracing" yaml:"tracing"`
}

type BaseDependenciesConfig struct {
	GameDataDir        string             `json:"game_data_dir" yaml:"game_data_dir"`               // 游戏数据目录
	SensitiveWordsFile string             `json:"sensitive_words_file" yaml:"sensitive_words_file"` // 敏感词文件
	NacosConf          net_conf.NacosConf `json:"nacos_conf" yaml:"nacos_conf"`                     // nacos配置
	OrmConf            []orm.Config       `json:"orm_instances" yaml:"orm_instances"`               // mysql配置
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

	// Legacy flat fields stay readable during the migration to grouped config.
	RegisterAddr       string             `json:"register_addr" yaml:"register_addr"`
	BusMQAddr          string             `json:"bus_mq_addr" yaml:"bus_mq_addr"`
	GameDataDir        string             `json:"game_data_dir" yaml:"game_data_dir"`
	SensitiveWordsFile string             `json:"sensitive_words_file" yaml:"sensitive_words_file"`
	NacosConf          net_conf.NacosConf `json:"nacos_conf" yaml:"nacos_conf"`
	OrmConf            []orm.Config       `json:"orm_instances" yaml:"orm_instances"`
	HTTPSigns          []http_sign.Config `json:"http_sign" yaml:"http_sign"`
	RestApiConf        []rest_api.Config  `json:"rest_api_config" yaml:"rest_api_config"`
	DbInstances        []redis.Config     `json:"db_instances" yaml:"db_instances"`
	Pprof              bool               `json:"pprof" yaml:"pprof"`
	AdminServer        AdminServerConfig  `json:"admin_server" yaml:"admin_server"`
	Tracing            RuntimeTracingConfig `json:"tracing" yaml:"tracing"`
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

	SelfBusId string `json:"self_bus_id" yaml:"self_bus_id"`
	LogDir    string `json:"log_dir" yaml:"log_dir"`
	LogLevel  string `json:"log_level" yaml:"log_level"`
}

type ConnRuntimeConfig struct {
	ListenPort int `json:"listen_port" yaml:"listen_port"`
}

type MainCapacityConfig struct {
	TransShardCount        int      `json:"trans_shard_count" yaml:"trans_shard_count"`
	RoleSyncPatchEnabled   bool     `json:"role_sync_patch_enabled" yaml:"role_sync_patch_enabled"`
	RoleSyncPatchAllowUids []uint64 `json:"role_sync_patch_allow_uids" yaml:"role_sync_patch_allow_uids"`
	RolePersistDebounceSec int      `json:"role_persist_debounce_sec" yaml:"role_persist_debounce_sec"`
}

type RoomCenterCapacityConfig struct {
	TransShardCount int `json:"trans_shard_count" yaml:"trans_shard_count"`
}

type WebRuntimeConfig struct {
	HttpServer web_gin.Config   `json:"http_server" yaml:"http_server"`
	GRPCServer GRPCServerConfig `json:"grpc_server" yaml:"grpc_server"`
}

type ConnSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Runtime             ConnRuntimeConfig `json:"runtime" yaml:"runtime"`

	ListenPort int `json:"listen_port" yaml:"listen_port"`
}

type InfoSvr struct {
	ServiceCommonConfig `yaml:",inline"`
}

type MainSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Capacity            MainCapacityConfig `json:"capacity" yaml:"capacity"`

	TransShardCount        int      `json:"trans_shard_count" yaml:"trans_shard_count"`
	RoleSyncPatchEnabled   bool     `json:"role_sync_patch_enabled" yaml:"role_sync_patch_enabled"`
	RoleSyncPatchAllowUids []uint64 `json:"role_sync_patch_allow_uids" yaml:"role_sync_patch_allow_uids"`
	RolePersistDebounceSec int      `json:"role_persist_debounce_sec" yaml:"role_persist_debounce_sec"`
}

type MySqlSvr struct {
	ServiceCommonConfig `yaml:",inline"`
}

type RoomCenterSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Capacity            RoomCenterCapacityConfig `json:"capacity" yaml:"capacity"`

	TransShardCount int `json:"trans_shard_count" yaml:"trans_shard_count"`
}

type WebSvr struct {
	ServiceCommonConfig `yaml:",inline"`
	Runtime             WebRuntimeConfig `json:"runtime" yaml:"runtime"`

	HttpServer web_gin.Config   `json:"http_server" yaml:"http_server"`
	GRPCServer GRPCServerConfig `json:"grpc_server" yaml:"grpc_server"`
}

type GRPCServerConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	IP      string `json:"ip" yaml:"ip"`
	Port    int    `json:"port" yaml:"port"`
}

// connsvr配置
type ConnConfig struct {
	BaseCfg `json:"base_cfg" yaml:"base_cfg"`
	ConnSvr `json:"connsvr" yaml:"connsvr"`
}

var ConnSvrCfg ConnConfig

// infosvr配置
type InfoConfig struct {
	BaseCfg `json:"base_cfg" yaml:"base_cfg"`
	InfoSvr `json:"infosvr" yaml:"infosvr"`
}

var InfoSvrCfg InfoConfig

// mainsvr配置
type MainSvrConfig struct {
	BaseCfg `json:"base_cfg" yaml:"base_cfg"`
	MainSvr `json:"mainsvr" yaml:"mainsvr"`
}

var MainSvrCfg MainSvrConfig

// mysqlsvr配置
type MySqlSvrConfig struct {
	BaseCfg  `json:"base_cfg" yaml:"base_cfg"`
	MySqlSvr `json:"mysqlsvr" yaml:"mysqlsvr"`
}

var MySqlSvrCfg MySqlSvrConfig

// roomcentersvr配置
type RoomCenterConfig struct {
	BaseCfg       `json:"base_cfg" yaml:"base_cfg"`
	RoomCenterSvr `json:"roomcentersvr" yaml:"roomcentersvr"`
}

var RoomCenterSvrCfg RoomCenterConfig

// websvr配置
type webSvrConfig struct {
	BaseCfg `json:"base_cfg" yaml:"base_cfg"`
	WebSvr  `json:"websvr" yaml:"websvr"`
}

var WebSvrCfg webSvrConfig

type configDocument interface {
	normalize()
	validate() error
}

func LoadConnConfig(file string) error {
	return loadConfig(file, &ConnSvrCfg)
}

func LoadInfoConfig(file string) error {
	return loadConfig(file, &InfoSvrCfg)
}

func LoadMainConfig(file string) error {
	return loadConfig(file, &MainSvrCfg)
}

func LoadMySQLConfig(file string) error {
	return loadConfig(file, &MySqlSvrCfg)
}

func LoadRoomCenterConfig(file string) error {
	return loadConfig(file, &RoomCenterSvrCfg)
}

func LoadWebConfig(file string) error {
	return loadConfig(file, &WebSvrCfg)
}

func loadConfig(file string, cfg configDocument) error {
	if err := marshal.LoadConfFile(file, cfg); err != nil {
		return err
	}
	cfg.normalize()
	if err := cfg.validate(); err != nil {
		return fmt.Errorf("validate config %s: %w", file, err)
	}
	return nil
}

func (c *BaseCfg) normalize() {
	c.CommonRuntime.RegisterAddr = coalesceString(c.CommonRuntime.RegisterAddr, c.RegisterAddr)
	c.CommonRuntime.BusMQAddr = coalesceString(c.CommonRuntime.BusMQAddr, c.BusMQAddr)
	c.CommonRuntime.AdminServer = mergeAdminServer(c.CommonRuntime.AdminServer, c.AdminServer)
	c.CommonRuntime.Tracing = coalesceStruct(c.CommonRuntime.Tracing, c.Tracing)

	c.Dependencies.GameDataDir = coalesceString(c.Dependencies.GameDataDir, c.GameDataDir)
	c.Dependencies.SensitiveWordsFile = coalesceString(c.Dependencies.SensitiveWordsFile, c.SensitiveWordsFile)
	c.Dependencies.NacosConf = coalesceStruct(c.Dependencies.NacosConf, c.NacosConf)
	c.Dependencies.OrmConf = coalesceSlice(c.Dependencies.OrmConf, c.OrmConf)
	c.Dependencies.HTTPSigns = coalesceSlice(c.Dependencies.HTTPSigns, c.HTTPSigns)
	c.Dependencies.RestApiConf = coalesceSlice(c.Dependencies.RestApiConf, c.RestApiConf)
	c.Dependencies.DbInstances = coalesceSlice(c.Dependencies.DbInstances, c.DbInstances)

	c.CommonDebug.Pprof = coalesceBool(c.CommonDebug.Pprof, c.Pprof)

	c.RegisterAddr = c.CommonRuntime.RegisterAddr
	c.BusMQAddr = c.CommonRuntime.BusMQAddr
	c.AdminServer = c.CommonRuntime.AdminServer
	c.Tracing = c.CommonRuntime.Tracing

	c.GameDataDir = c.Dependencies.GameDataDir
	c.SensitiveWordsFile = c.Dependencies.SensitiveWordsFile
	c.NacosConf = c.Dependencies.NacosConf
	c.OrmConf = c.Dependencies.OrmConf
	c.HTTPSigns = c.Dependencies.HTTPSigns
	c.RestApiConf = c.Dependencies.RestApiConf
	c.DbInstances = c.Dependencies.DbInstances
	c.Pprof = c.CommonDebug.Pprof
}

func (c *BaseCfg) validate() error {
	if c.CommonRuntime.AdminServer.Port < 0 {
		return fmt.Errorf("base_cfg.runtime.admin_server.port must be >= 0")
	}
	if c.CommonRuntime.Tracing.SamplerRatio < 0 || c.CommonRuntime.Tracing.SamplerRatio > 1 {
		return fmt.Errorf("base_cfg.runtime.tracing.sampler_ratio must be between 0 and 1")
	}
	return nil
}

func (c *BaseCfg) validateBusRuntime(service string) error {
	if strings.TrimSpace(c.CommonRuntime.RegisterAddr) == "" {
		return fmt.Errorf("%s base_cfg.runtime.register_addr is required", service)
	}
	if strings.TrimSpace(c.CommonRuntime.BusMQAddr) == "" {
		return fmt.Errorf("%s base_cfg.runtime.bus_mq_addr is required", service)
	}
	return nil
}

func (c *ServiceCommonConfig) normalize() {
	c.Identity.SelfBusId = coalesceString(c.Identity.SelfBusId, c.SelfBusId)
	c.Debug.LogDir = coalesceString(c.Debug.LogDir, c.LogDir)
	c.Debug.LogLevel = coalesceString(c.Debug.LogLevel, c.LogLevel)

	c.SelfBusId = c.Identity.SelfBusId
	c.LogDir = c.Debug.LogDir
	c.LogLevel = c.Debug.LogLevel
}

func (c *ServiceCommonConfig) validate(service string) error {
	if strings.TrimSpace(c.Identity.SelfBusId) == "" {
		return fmt.Errorf("%s.identity.self_bus_id is required", service)
	}
	return nil
}

func (c *ConnSvr) normalize() {
	c.ServiceCommonConfig.normalize()
	c.Runtime.ListenPort = coalesceInt(c.Runtime.ListenPort, c.ListenPort)
	c.ListenPort = c.Runtime.ListenPort
}

func (c *ConnSvr) validate() error {
	if err := c.ServiceCommonConfig.validate("connsvr"); err != nil {
		return err
	}
	if c.Runtime.ListenPort <= 0 {
		return fmt.Errorf("connsvr.runtime.listen_port must be > 0")
	}
	return nil
}

func (c *InfoSvr) normalize() {
	c.ServiceCommonConfig.normalize()
}

func (c *InfoSvr) validate() error {
	return c.ServiceCommonConfig.validate("infosvr")
}

func (c *MainSvr) normalize() {
	c.ServiceCommonConfig.normalize()
	c.Capacity.TransShardCount = coalesceInt(c.Capacity.TransShardCount, c.TransShardCount)
	c.Capacity.RoleSyncPatchEnabled = coalesceBool(c.Capacity.RoleSyncPatchEnabled, c.RoleSyncPatchEnabled)
	c.Capacity.RoleSyncPatchAllowUids = coalesceSlice(c.Capacity.RoleSyncPatchAllowUids, c.RoleSyncPatchAllowUids)
	c.Capacity.RolePersistDebounceSec = coalesceInt(c.Capacity.RolePersistDebounceSec, c.RolePersistDebounceSec)

	c.TransShardCount = c.Capacity.TransShardCount
	c.RoleSyncPatchEnabled = c.Capacity.RoleSyncPatchEnabled
	c.RoleSyncPatchAllowUids = c.Capacity.RoleSyncPatchAllowUids
	c.RolePersistDebounceSec = c.Capacity.RolePersistDebounceSec
}

func (c *MainSvr) validate() error {
	if err := c.ServiceCommonConfig.validate("mainsvr"); err != nil {
		return err
	}
	if c.Capacity.TransShardCount < 0 {
		return fmt.Errorf("mainsvr.capacity.trans_shard_count must be >= 0")
	}
	if c.Capacity.RolePersistDebounceSec < 0 {
		return fmt.Errorf("mainsvr.capacity.role_persist_debounce_sec must be >= 0")
	}
	return nil
}

func (c *MySqlSvr) normalize() {
	c.ServiceCommonConfig.normalize()
}

func (c *MySqlSvr) validate() error {
	return c.ServiceCommonConfig.validate("mysqlsvr")
}

func (c *RoomCenterSvr) normalize() {
	c.ServiceCommonConfig.normalize()
	c.Capacity.TransShardCount = coalesceInt(c.Capacity.TransShardCount, c.TransShardCount)
	c.TransShardCount = c.Capacity.TransShardCount
}

func (c *RoomCenterSvr) validate() error {
	if err := c.ServiceCommonConfig.validate("roomcentersvr"); err != nil {
		return err
	}
	if c.Capacity.TransShardCount < 0 {
		return fmt.Errorf("roomcentersvr.capacity.trans_shard_count must be >= 0")
	}
	return nil
}

func (c *WebSvr) normalize() {
	c.ServiceCommonConfig.normalize()
	c.Runtime.HttpServer = coalesceStruct(c.Runtime.HttpServer, c.HttpServer)
	c.Runtime.GRPCServer = coalesceStruct(c.Runtime.GRPCServer, c.GRPCServer)

	c.HttpServer = c.Runtime.HttpServer
	c.GRPCServer = c.Runtime.GRPCServer
}

func (c *WebSvr) validate() error {
	if err := c.ServiceCommonConfig.validate("websvr"); err != nil {
		return err
	}
	if c.Runtime.HttpServer.Port <= 0 {
		return fmt.Errorf("websvr.runtime.http_server.port must be > 0")
	}
	if c.Runtime.GRPCServer.Enabled && c.Runtime.GRPCServer.Port <= 0 {
		return fmt.Errorf("websvr.runtime.grpc_server.port must be > 0 when enabled")
	}
	return nil
}

func (c *ConnConfig) normalize() {
	c.BaseCfg.normalize()
	c.ConnSvr.normalize()
}

func (c *ConnConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.BaseCfg.validateBusRuntime("connsvr"); err != nil {
		return err
	}
	return c.ConnSvr.validate()
}

func (c *InfoConfig) normalize() {
	c.BaseCfg.normalize()
	c.InfoSvr.normalize()
}

func (c *InfoConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.BaseCfg.validateBusRuntime("infosvr"); err != nil {
		return err
	}
	if err := c.InfoSvr.validate(); err != nil {
		return err
	}
	if len(c.Dependencies.DbInstances) == 0 {
		return fmt.Errorf("infosvr base_cfg.dependencies.db_instances must not be empty")
	}
	return nil
}

func (c *MainSvrConfig) normalize() {
	c.BaseCfg.normalize()
	c.MainSvr.normalize()
}

func (c *MainSvrConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.BaseCfg.validateBusRuntime("mainsvr"); err != nil {
		return err
	}
	if err := c.MainSvr.validate(); err != nil {
		return err
	}
	if len(c.Dependencies.DbInstances) == 0 {
		return fmt.Errorf("mainsvr base_cfg.dependencies.db_instances must not be empty")
	}
	return nil
}

func (c *MySqlSvrConfig) normalize() {
	c.BaseCfg.normalize()
	c.MySqlSvr.normalize()
}

func (c *MySqlSvrConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.BaseCfg.validateBusRuntime("mysqlsvr"); err != nil {
		return err
	}
	if err := c.MySqlSvr.validate(); err != nil {
		return err
	}
	if len(c.Dependencies.OrmConf) == 0 {
		return fmt.Errorf("mysqlsvr base_cfg.dependencies.orm_instances must not be empty")
	}
	return nil
}

func (c *RoomCenterConfig) normalize() {
	c.BaseCfg.normalize()
	c.RoomCenterSvr.normalize()
}

func (c *RoomCenterConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.BaseCfg.validateBusRuntime("roomcentersvr"); err != nil {
		return err
	}
	return c.RoomCenterSvr.validate()
}

func (c *webSvrConfig) normalize() {
	c.BaseCfg.normalize()
	c.WebSvr.normalize()
}

func (c *webSvrConfig) validate() error {
	if err := c.BaseCfg.validate(); err != nil {
		return err
	}
	if err := c.WebSvr.validate(); err != nil {
		return err
	}
	if len(c.Dependencies.DbInstances) == 0 {
		return fmt.Errorf("websvr base_cfg.dependencies.db_instances must not be empty")
	}
	return nil
}


func coalesceString(current, legacy string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return legacy
}

func coalesceInt(current, legacy int) int {
	if current != 0 {
		return current
	}
	return legacy
}

func coalesceBool(current, legacy bool) bool {
	return current || legacy
}

func coalesceSlice[T any](current, legacy []T) []T {
	if len(current) > 0 {
		return current
	}
	return legacy
}

func coalesceStruct[T any](current, legacy T) T {
	if !reflect.ValueOf(current).IsZero() {
		return current
	}
	return legacy
}

func mergeAdminServer(current, legacy AdminServerConfig) AdminServerConfig {
	current.Enabled = coalesceBool(current.Enabled, legacy.Enabled)
	current.IP = coalesceString(current.IP, legacy.IP)
	current.Port = coalesceInt(current.Port, legacy.Port)
	return current
}
