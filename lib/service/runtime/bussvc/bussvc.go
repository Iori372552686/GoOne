// Package bussvc 提供 bus 服务（connsvr/mainsvr/infosvr/mysqlsvr/roomcentersvr）在
// runtime.App 之上的标准 Component 装配，取代旧的 bootstrap/busapp。
//
// 它把五个 bus 服务共用的接线（tracing 初始化、TransactionMgr 启动、router 启动、
// 就绪探针、graceful shutdown 顺序）拆成可独立 Start/Stop 的 Component，注册到
// runtime.App 后由统一生命周期驱动。
package bussvc

import (
	"context"
	"errors"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
)

// ErrDriversNotConfigured 在 RouterComponent.Drivers 为 nil 时由 Start 返回。
// 历史实现回退到 driver/all 的包级 bus.CreateBus，使所有 bus 服务都隐式链接全部 5 类
// MQ SDK。生产 bus 服务必须显式装配 DriverRegistry（通常只注册 rabbitmq）；websvr 不装
// 配任何 bus。
var ErrDriversNotConfigured = errors.New("bussvc: RouterComponent.Drivers not configured; bus services must explicitly register drivers")

// Common 承载每个 bus 服务共享的配置段，由服务的配置访问器在 LoadConfig 后产出。
type Common struct {
	LogDir   string
	LogLevel string

	SelfBusId    string
	BusMQAddr    string
	RegisterAddr string

	AdminEnabled bool
	AdminIP      string
	AdminPort    int
	Pprof        bool

	Tracing ssrpc.TracingConfig
}

// CommonFromConf 从 conf 按 key 读取并组装 Common，一处实现取代历史上散落在各
// 服务 app.go 的 5 份近全同 xxxCommon()。svc 是服务段名（如 "connsvr"）。
//
// 读取的 key 路径与 yaml 结构一致：
//   - <svc>.debug.log_dir / log_level
//   - <svc>.identity.self_bus_id
//   - base_cfg.runtime.bus_mq_addr / register_addr / admin_server.* / tracing.*
//   - base_cfg.debug.pprof
//
// admin 端口为 0 时按服务类型回退（conf.ResolveAdminPort）。
func CommonFromConf(svc string) Common {
	admPrefix := "base_cfg.runtime.admin_server."
	admPort := conf.ResolveAdminPort(conf.Get(admPrefix+"port").Int(), svc)
	trPrefix := "base_cfg.runtime.tracing."
	return Common{
		LogDir:       conf.Get(svc + ".debug.log_dir").String(),
		LogLevel:     conf.Get(svc + ".debug.log_level").String(),
		SelfBusId:    conf.Get(svc + ".identity.self_bus_id").String(),
		BusMQAddr:    conf.Get("base_cfg.runtime.bus_mq_addr").String(),
		RegisterAddr: conf.Get("base_cfg.runtime.register_addr").String(),
		AdminEnabled: conf.Get(admPrefix + "enabled").Bool(),
		AdminIP:      conf.Get(admPrefix + "ip").String(),
		AdminPort:    admPort,
		Pprof:        conf.Get("base_cfg.debug.pprof").Bool(),
		Tracing: ssrpc.TracingConfig{
			Enabled:      conf.Get(trPrefix + "enabled").Bool(),
			Exporter:     conf.Get(trPrefix + "exporter").String(),
			Endpoint:     conf.Get(trPrefix + "endpoint").String(),
			Insecure:     conf.Get(trPrefix + "insecure").Bool(),
			SamplerRatio: conf.Get(trPrefix + "sampler_ratio").Float64(),
			Headers:      conf.Get(trPrefix + "headers").StringsMap(),
		},
	}
}

// TracingComponent 把 ssrpc trace 的初始化与关闭包成 Component（早启动、晚停止）。
type TracingComponent struct {
	ServiceName string
	Cfg         func() ssrpc.TracingConfig
	inited      bool
}

// Name 实现 runtime.Component。
func (t *TracingComponent) Name() string { return "tracing" }

// Start 实现 runtime.Component：初始化 tracing provider。
func (t *TracingComponent) Start(_ context.Context) error {
	if t.Cfg == nil {
		return nil
	}
	if err := ssrpc.InitTracing(t.ServiceName, t.Cfg()); err != nil {
		return err
	}
	t.inited = true
	return nil
}

// Stop 实现 runtime.Component：关闭 tracing provider。
func (t *TracingComponent) Stop(ctx context.Context) error {
	if !t.inited {
		return nil
	}
	return ssrpc.ShutdownTracing(ctx)
}

// LoggerConfig 承载 logger 初始化参数（与旧 bootstrap.LoggerConfig 对应）。
type LoggerConfig struct {
	Dir   string
	Level string
	Name  string
}

// LoggerComponent 把 logger 的初始化包成 Component（最早启动、最晚停止）。Stop 时
// Flush，确保停机前日志落盘。
type LoggerComponent struct {
	Cfg    func() LoggerConfig
	inited bool
}

// Name 实现 runtime.Component。
func (l *LoggerComponent) Name() string { return "logger" }

// Start 实现 runtime.Component：初始化 logger。
func (l *LoggerComponent) Start(_ context.Context) error {
	if l.Cfg == nil {
		return nil
	}
	c := l.Cfg()
	if _, err := logger.InitLogger(c.Dir, c.Level, c.Name); err != nil {
		return err
	}
	l.inited = true
	return nil
}

// Stop 实现 runtime.Component：Flush 日志。
func (l *LoggerComponent) Stop(_ context.Context) error {
	if l.inited {
		logger.Flush()
	}
	return nil
}

// TransMgrComponent 把 TransactionMgr 的启动与排空关闭包成 Component。它实现
// Drainer：Close 排空在途事务（受 ctx 超时约束）。
//
// 配置为值语义（无 func 包装）：NewApp 装配期 conf 尚未 Load，故分片数不在装配期
// 读 conf；改用 ShardCountConfKey 声明 key（如 "mainsvr.capacity.trans_shard_count"），
// Start 时（LoadConfig 之后）读取，缺省或 <=0 内部回退 DefaultShardCount()。
// Cfg 与 ShardCountConfKey 全零时走遗留单分片路径（connsvr/infosvr/mysqlsvr 默认）。
type TransMgrComponent struct {
	Mgr transaction.ITransactionMgr
	// Cfg 静态配置（MaxTrans / MaxPendingPerKey 直接给值）。MaxTrans<=0 内部回退
	// misc.MaxTransNumber；ShardCount 一般不在这里给，用 ShardCountConfKey。
	Cfg transaction.TransactionMgrConfig
	// ShardCountConfKey 非空时，Start 从 conf 读取分片数覆盖 Cfg.ShardCount；
	// <=0 或 key 不存在时内部回退 transaction.DefaultShardCount()。
	ShardCountConfKey string
	started           bool
}

// Name 实现 runtime.Component。
func (c *TransMgrComponent) Name() string { return "transaction_mgr" }

// Start 实现 runtime.Component：解析配置并启动 TransactionMgr。
// 全零配置走遗留单分片路径（行为与历史 InitAndRun(misc.MaxTransNumber, false, 0)
// 完全一致）；否则按值语义配置启动，ShardCount/MaxTrans 的 <=0 回退在内部完成。
func (c *TransMgrComponent) Start(_ context.Context) error {
	if c.ShardCountConfKey == "" && c.Cfg == (transaction.TransactionMgrConfig{}) {
		c.Mgr.InitAndRun(misc.MaxTransNumber, false, 0)
		c.started = true
		return nil
	}
	cfg := c.Cfg
	if c.ShardCountConfKey != "" {
		cfg.ShardCount = conf.Get(c.ShardCountConfKey).Int()
	}
	if cfg.MaxTrans <= 0 {
		cfg.MaxTrans = misc.MaxTransNumber
	}
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = transaction.DefaultShardCount()
	}
	logger.Infof("transmgr shards=%d max_pending_per_key=%d serial_key=routerid_or_uid", cfg.ShardCount, cfg.MaxPendingPerKey)
	c.Mgr.InitAndRunWithConfig(cfg)
	c.started = true
	return nil
}

// Drain 实现 runtime.Drainer：排空在途事务。在 router 注销之后、router.Close 之前
// 执行（由注册顺序保证）。
func (c *TransMgrComponent) Drain(ctx context.Context) error {
	if !c.started {
		return nil
	}
	return c.Mgr.Close(ctx)
}

// Stop 实现 runtime.Component：幂等（Close 已在 Drain 执行；此处兜底）。
func (c *TransMgrComponent) Stop(_ context.Context) error {
	return nil
}

// RouterComponent 把 router（bus + 服务注册发现）的启动与关闭包成 Component。它实现
// Quiescer：BeginShutdown 注销实例并拒绝新请求；仍接收 DstTransID 非零的事务响应。
type RouterComponent struct {
	Common         func() Common
	OnRecvSSPacket func(*sharedstruct.SSPacket) // 可选；为 nil 时默认投到 TransMgr。
	TransMgr       transaction.ITransactionMgr
	// Drivers 是显式 Driver 注册表（必填）。bus 服务在装配期创建
	// bus.NewDriverRegistry() 并 MustRegister 所需 driver（通常仅 rabbitmq）；Start 用它
	// 创建 bus，只链接注册的 driver。nil 时 Start 返回 ErrDriversNotConfigured（不再
	// 回退到 driver/all）。websvr 不装配 bus，故不创建 RouterComponent。
	Drivers *bus.DriverRegistry
}

// Name 实现 runtime.Component。
func (r *RouterComponent) Name() string { return "router_bus" }

// Start 实现 runtime.Component：启动 router（含 bus 连接与服务注册）。
//
// Drivers 必须显式装配（bus 服务只链接选定的 driver，通常 rabbitmq）。nil
// Drivers 返回 ErrDriversNotConfigured。历史实现的 driver/all 回退已删除。
func (r *RouterComponent) Start(_ context.Context) error {
	if r.Drivers == nil {
		return ErrDriversNotConfigured
	}
	c := r.Common()
	onRecv := r.OnRecvSSPacket
	if onRecv == nil {
		mgr := r.TransMgr
		onRecv = func(packet *sharedstruct.SSPacket) {
			mgr.ProcessSSPacket(packet)
		}
	}
	selfBusInt := bus.IpStringToInt(c.SelfBusId)
	addr := c.BusMQAddr
	drivers := r.Drivers
	busCtor := func(onRecvMsg bus.MsgHandler) (bus.IBus, error) {
		return drivers.CreateBus(selfBusInt, onRecvMsg, addr)
	}
	return router.InitAndRunWithBusCtor(c.SelfBusId, onRecv, busCtor, misc.ServerRouteRules, c.RegisterAddr)
}

// Quiesce 实现 runtime.Quiescer：从服务注册中心注销，admission gate 拒绝新请求，但仍
// 接收事务响应。
func (r *RouterComponent) Quiesce(_ context.Context) error {
	router.BeginShutdown()
	return nil
}

// Stop 实现 runtime.Component：关闭 bus 连接、producer、watcher。
func (r *RouterComponent) Stop(_ context.Context) error {
	return router.Close()
}

// ReadyCheckComponent 把 router.ReadyCheck 包成一个可被 StateObserver 使用的就绪探针。
// 当 bus 断连时返回非 nil error，使 /readyz 返回 503 自动摘流。
//
// 注意：runtime.App 的 Ready 状态由 StateObserver 在 Ready 转换时调用；本组件提供一个
// 便捷的 Observer 实现，把 router 健康纳入就绪判定。
type ReadyCheckComponent struct {
	Check func() error
}

// Name 实现 runtime.Component（占位；本身不持有资源）。
func (r *ReadyCheckComponent) Name() string { return "ready_check" }

// Start 实现 runtime.Component。
func (r *ReadyCheckComponent) Start(_ context.Context) error { return nil }

// Stop 实现 runtime.Component。
func (r *ReadyCheckComponent) Stop(_ context.Context) error { return nil }

// FuncComponent 把任意 Start/Stop 闭包包成 Component，用于服务特有的一次性装配
// （redis/orm/idgen/gamedata 初始化、SelfLogoutSender 注入等）。
type FuncComponent struct {
	ComponentName string
	OnStart       func(ctx context.Context) error
	OnStop        func(ctx context.Context) error
}

// Name 实现 runtime.Component。
func (f *FuncComponent) Name() string { return f.ComponentName }

// Start 实现 runtime.Component。
func (f *FuncComponent) Start(ctx context.Context) error {
	if f.OnStart == nil {
		return nil
	}
	return f.OnStart(ctx)
}

// Stop 实现 runtime.Component。
func (f *FuncComponent) Stop(ctx context.Context) error {
	if f.OnStop == nil {
		return nil
	}
	return f.OnStop(ctx)
}

// 暂存的聚合错误工具（避免各服务重复 import errors.Join）。
func joinErrs(errs ...error) error {
	return errors.Join(errs...)
}

var _ = logger.Infof // 保留 logger import 供未来组件使用；当前组件直接走 router/ssrpc。
var _ runtime.Component = (*TracingComponent)(nil)
var _ runtime.Component = (*TransMgrComponent)(nil)
var _ runtime.Quiescer = (*RouterComponent)(nil)
var _ runtime.Component = (*RouterComponent)(nil)
var _ runtime.Drainer = (*TransMgrComponent)(nil)
