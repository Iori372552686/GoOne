package bussvc

import (
	"context"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

// Assembly supplies application-specific configuration to the standard components.
// Callbacks are evaluated during startup, after the application has registered its components.
type Assembly struct {
	LoadServiceConfig func(service string) error
	CommonForService  func(service string) Common
	RouteRules        func() map[uint32]uint32
}

func (b Assembly) common(service string) Common {
	if b.CommonForService == nil {
		return Common{}
	}
	return b.CommonForService(service)
}

// LoadConf loads service configuration and runs hooks in registration order.
func (b Assembly) LoadConf(svc string, hooks ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		if b.LoadServiceConfig != nil {
			if err := b.LoadServiceConfig(svc); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for %s", svc)
		}
		for i, hook := range hooks {
			if hook == nil {
				continue
			}
			if err := hook(ctx); err != nil {
				return fmt.Errorf("bussvc: %s 配置加载钩子 %d 失败: %w", svc, i, err)
			}
		}
		return nil
	}
}

// WithConfLoader sets the App's configuration loader and binds it to the app name.
func (b Assembly) WithConfLoader(hooks ...func(context.Context) error) runtime.Option {
	return func(app *runtime.App) {
		runtime.WithLoadConfig(b.LoadConf(app.Name(), hooks...))(app)
	}
}

// MustNew 是 bus 服务的统一装配入口：构建 runtime.App 后集中注册 4 个标准组件
// （datetime_tick → logger → admin → tracing，顺序即 Start 顺序）。
//
// 它取代历史上散落在各 app.go 内的 "NewLoggerComponent/NewAdminComponent/
// NewTracingComponent + datetime.DefaultDateTimeTick + MustRegister" 样板。
// runtime.MustNew 仅构建 App，不装配组件——标准组件集中装配属于 bussvc（上层），
// 不能下沉到 runtime（底层，否则 runtime→bussvc→runtime 循环 import）。
//
// readyCheck 是 admin 运行期就绪探针：bus 服务传 router.ReadyCheck（bus 断连时
// /readyz 返 503 自动摘流），无 bus 的服务（websvr）传 nil。二者差异刻意保留，
// 避免给无 bus 的服务挂上恒失败的探针。
//
// 用法：
//
//	app := assembly.MustNew("connsvr", router.ReadyCheck, assembly.WithConfLoader())
//
// opts 在 runtime.MustNew 内先于组件构造应用，故 WithConfLoader 等配置 Option 正常生效。
func (b Assembly) MustNew(name string, readyCheck func() error, opts ...runtime.Option) *runtime.App {
	app := runtime.MustNew(name, opts...)
	app.MustRegister(
		NewDateTimeComponent(),
		b.NewLoggerComponent(app),
		b.NewAdminComponent(app, readyCheck),
		b.NewTracingComponent(app),
	)
	return app
}

// NewLoggerComponent 构建标准 logger 组件：日志目录/级别在 Start 时读取
// "<svc>.debug.log_dir / log_level"，日志名即服务名（最早启动、最晚停止，Stop 时
// Flush 落盘）。
func (b Assembly) NewLoggerComponent(app *runtime.App) *LoggerComponent {
	svc := mustAppName(app)
	return &LoggerComponent{
		Cfg: func() LoggerConfig {
			c := b.common(svc)
			return LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: svc}
		},
	}
}

// NewAdminComponent 构建标准 admin 组件：Enabled/IP/Port/Pprof 在 Start 时经
// 装配时传入的配置源读取；
// serviceName 由 runtime 默认取 app.Name()，无需重复标注。
//
// readyCheck 是运行期就绪探针：bus 服务传 router.ReadyCheck，bus 断连时 /readyz
// 返回 503 自动摘流；无探针的服务（websvr）传 nil。
func (b Assembly) NewAdminComponent(app *runtime.App, readyCheck func() error) *runtime.AdminComponent {
	svc := mustAppName(app)
	opts := []runtime.AdminOption{
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := b.common(svc)
			return runtime.AdminConfig{
				Enabled: c.AdminEnabled,
				IP:      c.AdminIP,
				Port:    c.AdminPort,
				Pprof:   c.Pprof,
			}
		}),
	}
	if readyCheck != nil {
		opts = append(opts, runtime.WithAdminReadyCheck(readyCheck))
	}
	return runtime.NewAdminComponent(app, opts...)
}

// NewTracingComponent 构建标准 tracing 组件：Start 时读取
// base_cfg.runtime.tracing.*，服务名取 app.Name()（早启动、晚停止）。
func (b Assembly) NewTracingComponent(app *runtime.App) *TracingComponent {
	svc := mustAppName(app)
	return &TracingComponent{
		ServiceName: svc,
		Cfg:         func() ssrpc.TracingConfig { return b.common(svc).Tracing },
	}
}

// NewDateTimeComponent 构建标准 datetime 刷新组件：返回驱动 datetime 缓存刷新的
// scheduler.Task（Inline 模式，周期 datetime.TickInterval()，默认 100ms）。
// 实际逻辑委托给 datetime.DefaultDateTimeTick（datetime 包内联 Task 工厂），本构造器
// 仅做风格收敛，使调用方写 bussvc.NewDateTimeComponent(app) 而非裸调
func NewDateTimeComponent() runtime.Component {
	t := scheduler.New(
		"datetime_tick",
		datetime.TickInterval(),
		func(_ context.Context) error {
			datetime.Tick()
			return nil
		},
	)
	t.Inline = true
	return t
}

// NewRouterComponent 构建标准 router/bus 组件：Common（self_bus_id、bus_mq_addr、
// register_addr）在 Start 时从装配时传入的配置源读取。
//
// transMgr 与 drivers 是服务相关参数，保持显式传入：drivers 必须显式装配（通常
// rabbitmq.NewRegistry()，只链接选定 driver），nil 时 Start 返回
// ErrDriversNotConfigured。onRecvSSPacket 可选，传入时覆盖默认的
// TransMgr.ProcessSSPacket 投递（connsvr 网关用于下行包短路回客户端）；至多取第一个。
func (b Assembly) NewRouterComponent(app *runtime.App, transMgr transaction.ITransactionMgr, drivers *bus.DriverRegistry, onRecvSSPacket ...func(*sharedstruct.SSPacket)) *RouterComponent {
	svc := mustAppName(app)
	var onRecv func(*sharedstruct.SSPacket)
	if len(onRecvSSPacket) > 0 {
		onRecv = onRecvSSPacket[0]
	}
	return &RouterComponent{
		Common:         func() Common { return b.common(svc) },
		RouteRules:     b.RouteRules,
		OnRecvSSPacket: onRecv,
		TransMgr:       transMgr,
		Drivers:        drivers,
	}
}

// mustAppName 返回 app 的服务名。app 为 nil 时 panic——装配错误 fail-fast，与
// runtime.MustNew/MustRegister 的语义一致（服务名非空由 runtime.New 保证）。
func mustAppName(app *runtime.App) string {
	if app == nil {
		panic("bussvc: 标准组件构造器收到 nil *runtime.App（先用 runtime.MustNew 创建 app）")
	}
	return app.Name()
}
