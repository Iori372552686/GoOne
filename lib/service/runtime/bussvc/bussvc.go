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
	"github.com/Iori372552686/GoOne/module/misc"
)

// ErrDriversNotConfigured 在 RouterComponent.Drivers 为 nil 时由 Start 返回（P1-04）。
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
type TransMgrComponent struct {
	Mgr     transaction.ITransactionMgr
	Cfg     func() transaction.TransactionMgrConfig // 可选；为 nil 时用默认单分片。
	started bool
}

// Name 实现 runtime.Component。
func (c *TransMgrComponent) Name() string { return "transaction_mgr" }

// Start 实现 runtime.Component：按配置启动 TransactionMgr。
func (c *TransMgrComponent) Start(_ context.Context) error {
	if c.Cfg != nil {
		c.Mgr.InitAndRunWithConfig(c.Cfg())
	} else {
		c.Mgr.InitAndRun(misc.MaxTransNumber, false, 0)
	}
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
	// Drivers 是显式 Driver 注册表（P1-04：必填）。bus 服务在装配期创建
	// bus.NewDriverRegistry() 并 MustRegister 所需 driver（通常仅 rabbitmq）；Start 用它
	// 创建 bus，只链接注册的 driver。nil 时 Start 返回 ErrDriversNotConfigured（不再
	// 回退到 driver/all）。websvr 不装配 bus，故不创建 RouterComponent。
	Drivers *bus.DriverRegistry
}

// Name 实现 runtime.Component。
func (r *RouterComponent) Name() string { return "router_bus" }

// Start 实现 runtime.Component：启动 router（含 bus 连接与服务注册）。
//
// P1-04：Drivers 必须显式装配（bus 服务只链接选定的 driver，通常 rabbitmq）。nil
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
