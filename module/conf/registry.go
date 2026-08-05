package conf

import (
	"fmt"
	"strings"
)

// 本文件把历史上散落在 gconf/config.go 的 6 套 normalize/validate 集中为一处
// 注册式校验。每个服务用 Register 注册一个 Validator；Load 之后由 RunValidators
// 按 service 名调度。校验直接作用在已发布的快照 map 上，conf 包因此自包含、不依赖
// gconf struct（gconf 的 struct 类型后续 P2 改为从 conf 取值填充）。

// 默认 admin 端口表：admin_server.port 为 0 时按服务类型回退（8100 + ServerType）。
// ServerType 复用 module/misc 的命名常量，消除历史魔法 int 1/2/3/4/11/12。
//
// 端口分配：connsvr→8101 mainsvr→8102 infosvr→8103 mysqlsvr→8104
// roomcentersvr→8111 websvr→8112。
var defaultAdminPortByServerType = map[int]int{
	serverTypeConn:       8101,
	serverTypeMain:       8102,
	serverTypeInfo:       8103,
	serverTypeMysql:      8104,
	serverTypeRoomCenter: 8111,
	serverTypeWeb:        8112,
}

// module/misc 命名常量的本地别名，避免 conf 包为校验而引入整个 misc 包
//（misc 含路由规则等大量无关符号）。保持数值与 misc.ServerType_* 一致。
const (
	serverTypeConn       = 1
	serverTypeMain       = 2
	serverTypeInfo       = 3
	serverTypeMysql      = 4
	serverTypeRoomCenter = 11
	serverTypeWeb        = 12
)

// Validator 校验某个服务的配置快照。在 Load 后、服务装配前调用。
// 返回 error 时服务拒绝启动。normalize（如 admin 端口回退）已在 Load 期对快照
// 原地不可变快照的合法子集做改写——由于启动不可变，此处只做校验，normalize 的
// 端口回退由调用方读取时通过 ResolveAdminPort 提供，避免改写快照。
type Validator func(service string) error

var validators = map[string]Validator{}

// Register 为 service 注册校验器。重复注册 panic（装配期发现错误，fail-fast）。
func Register(service string, v Validator) {
	if _, dup := validators[service]; dup {
		panic(fmt.Sprintf("conf: 服务 %q 的校验器重复注册", service))
	}
	validators[service] = v
}

// RunValidators 运行 service 的已注册校验器。未注册时返回 nil（放行）。
// 必须在 Load 成功后调用。
func RunValidators(service string) error {
	if !loaded() {
		return ErrNotLoaded
	}
	v, ok := validators[service]
	if !ok {
		return nil
	}
	return v(service)
}

// ResolveAdminPort 按 service 类型回退 admin 端口。port 非 0 直接返回；
// 为 0 时按 defaultAdminPortByServerType 回退；未知服务保持 0（由 OS 分配或校验拒绝）。
// 供 app.go 构造 AdminConfig 时调用，取代历史上散落 6 处的 resolveAdminPort 调用。
func ResolveAdminPort(port int, service string) int {
	if port != 0 {
		return port
	}
	if def, ok := defaultAdminPortByServerType[serverTypeOf(service)]; ok {
		return def
	}
	return port
}

func serverTypeOf(service string) int {
	switch service {
	case "connsvr":
		return serverTypeConn
	case "mainsvr":
		return serverTypeMain
	case "infosvr":
		return serverTypeInfo
	case "mysqlsvr":
		return serverTypeMysql
	case "roomcentersvr":
		return serverTypeRoomCenter
	case "websvr":
		return serverTypeWeb
	default:
		return 0
	}
}

// ---- 公共校验原语：从快照读 key，对常见约束做断言。6 个服务复用，消除重复。 ----

func mustStr(service, key string) (string, error) {
	v := Get(key)
	if !v.Exists() || strings.TrimSpace(v.String()) == "" {
		return "", fmt.Errorf("%s.%s is required", service, key)
	}
	return v.String(), nil
}

func mustPositiveInt(service, key string) error {
	if v := Get(key); !v.Exists() || v.Int() <= 0 {
		return fmt.Errorf("%s.%s must be > 0", service, key)
	}
	return nil
}

func mustNonNegative(service, key string) error {
	if v := Get(key); v.Exists() && v.Int() < 0 {
		return fmt.Errorf("%s.%s must be >= 0", service, key)
	}
	return nil
}

func mustNonEmptyList(service, key string) error {
	if v := Get(key); !v.Exists() || len(v.Raw().([]any)) == 0 {
		return fmt.Errorf("%s.%s must not be empty", service, key)
	}
	return nil
}

func init() {
	// base_cfg 公共校验：admin 端口范围、tracing 采样率区间。所有服务都跑。
	// 这里按服务分别注册（RunValidators 接 service 名），每个 validator 内部复用
	// 原语，避免历史上 6 套 validate() 的逐字重复。
	for _, svc := range []string{"connsvr", "infosvr", "mainsvr", "mysqlsvr", "roomcentersvr", "websvr"} {
		svc := svc
		Register(svc, func(service string) error { return validateService(service) })
	}
}

// validateService 是各服务校验的总入口。按 service 名分支做服务特有校验，
// 公共部分（base_cfg）由 validateBase 处理。
func validateService(service string) error {
	if err := validateBase(service); err != nil {
		return err
	}
	// identity.self_bus_id 所有服务必填。
	if _, err := mustStr(service, service+".identity.self_bus_id"); err != nil {
		return err
	}
	switch service {
	case "connsvr":
		return validateConn()
	case "mainsvr":
		return validateMain()
	case "infosvr":
		return validateInfo()
	case "mysqlsvr":
		return validateMysql()
	case "roomcentersvr":
		return validateRoomCenter()
	case "websvr":
		return validateWeb()
	}
	return nil
}

// validateBase 校验 base_cfg 段：admin 端口范围、tracing 采样率、（bus 服务）
// register_addr/bus_mq_addr 必填。websvr 不要求 bus 段。
func validateBase(service string) error {
	if err := mustNonNegative(service, "base_cfg.runtime.admin_server.port"); err != nil {
		return err
	}
	sr := Get("base_cfg.runtime.tracing.sampler_ratio")
	if sr.Exists() {
		f := sr.Float64()
		if f < 0 || f > 1 {
			return fmt.Errorf("base_cfg.runtime.tracing.sampler_ratio must be between 0 and 1")
		}
	}
	if service != "websvr" {
		if _, err := mustStr(service, "base_cfg.runtime.register_addr"); err != nil {
			return err
		}
		if _, err := mustStr(service, "base_cfg.runtime.bus_mq_addr"); err != nil {
			return err
		}
	}
	return nil
}

func validateConn() error {
	if err := mustPositiveInt("connsvr", "connsvr.runtime.listen_port"); err != nil {
		return err
	}
	cap := "connsvr.capacity"
	for _, k := range []string{"max_connections", "max_unauthenticated_connections", "connection_rate", "login_rate", "max_inflight"} {
		if err := mustNonNegative("connsvr", cap+"."+k); err != nil {
			return err
		}
	}
	maxConn := Get(cap + ".max_connections").Int64()
	maxUnauth := Get(cap + ".max_unauthenticated_connections").Int64()
	if maxConn > 0 && maxUnauth > 0 && maxUnauth > maxConn {
		return fmt.Errorf("connsvr.capacity.max_unauthenticated_connections (%d) 不得超过 max_connections (%d)", maxUnauth, maxConn)
	}
	switch Get(cap + ".overload_mode").String() {
	case "", "off", "shadow", "enforce":
	default:
		return fmt.Errorf("connsvr.capacity.overload_mode 取值必须为 off/shadow/enforce")
	}
	return nil
}

func validateMain() error {
	if err := mustNonNegative("mainsvr", "mainsvr.capacity.trans_shard_count"); err != nil {
		return err
	}
	if err := mustNonNegative("mainsvr", "mainsvr.capacity.role_persist_debounce_sec"); err != nil {
		return err
	}
	if err := mustNonEmptyList("mainsvr", "base_cfg.dependencies.db_instances"); err != nil {
		return err
	}
	return nil
}

func validateInfo() error {
	return mustNonEmptyList("infosvr", "base_cfg.dependencies.db_instances")
}

func validateMysql() error {
	return mustNonEmptyList("mysqlsvr", "base_cfg.dependencies.orm_instances")
}

func validateRoomCenter() error {
	return mustNonNegative("roomcentersvr", "roomcentersvr.capacity.trans_shard_count")
}

func validateWeb() error {
	if err := mustPositiveInt("websvr", "websvr.runtime.http_server.port"); err != nil {
		return err
	}
	if Get("websvr.runtime.grpc_server.enabled").Bool() {
		if err := mustPositiveInt("websvr", "websvr.runtime.grpc_server.port"); err != nil {
			return err
		}
	}
	return mustNonEmptyList("websvr", "base_cfg.dependencies.db_instances")
}
