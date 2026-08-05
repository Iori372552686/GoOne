package bussvc

// 标准组件构造器（Due 风格装配）。
//
// 设计目标：服务名只在 runtime.MustNew 出现一次，标准组件（logger/admin/tracing/
// router）以 *runtime.App 为唯一上下文自行装配——服务名取 app.Name()，配置在组件
// Start 时（LoadConfig 之后）经 CommonFromConf 从 module/conf 读取。装配代码
// （src/<service>/app.go）不再显式传任何配置闭包。
//
// 与结构体字面量风格的关系：本文件只是便捷构造器，LoggerComponent.Cfg、
// RouterComponent.Common 等导出字段全部保留，测试与特殊场景仍可显式注入。
//
// 组件注册顺序仍由各 app.go 的 app.MustRegister(...) 显式控制——顺序即生命周期
// 语义，本文件不做自动排序。

import (
	"context"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/conf"
)

// LoadConf 返回标准 LoadConfig 闭包：conf.Load(conf.Path) 发布不可变快照，随后
// conf.RunValidators(svc) 运行该服务的注册校验器。hooks 是服务特有的追加加载逻辑
// （gamedata 本地目录、nacos 远端等），在校验通过后按序执行；任一 hook 失败即中止
// 启动，错误携带服务名与 hook 序号。
//
// 直接用于 runtime.WithLoadConfig；多数服务应改用 WithConfLoader（svc 自动取
// app.Name()）。本函数导出以便测试与自定义加载顺序的场景复用。
func LoadConf(svc string, hooks ...func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := conf.Load(conf.Path); err != nil {
			return err
		}
		if err := conf.RunValidators(svc); err != nil {
			return err
		}
		logger.Infof("svr_conf loaded for %s", svc)
		for i, h := range hooks {
			if h == nil {
				continue
			}
			if err := h(ctx); err != nil {
				return fmt.Errorf("bussvc: %s 配置加载钩子 %d 失败: %w", svc, i, err)
			}
		}
		return nil
	}
}

// WithConfLoader 是标准 LoadConfig 的 runtime.Option 形态，服务名取 app.Name()
// （runtime.New 先设置 name 再应用 Option，读取安全）。用法：
//
//	app := runtime.MustNew("connsvr", bussvc.WithConfLoader(gamedataHook))
//
// 不要与 runtime.WithLoadConfig 同时使用（后者会被覆盖）。
func WithConfLoader(hooks ...func(ctx context.Context) error) runtime.Option {
	return func(a *runtime.App) {
		runtime.WithLoadConfig(LoadConf(a.Name(), hooks...))(a)
	}
}

// NewLoggerComponent 构建标准 logger 组件：日志目录/级别在 Start 时读取
// "<svc>.debug.log_dir / log_level"，日志名即服务名（最早启动、最晚停止，Stop 时
// Flush 落盘）。
func NewLoggerComponent(app *runtime.App) *LoggerComponent {
	svc := mustAppName(app)
	return &LoggerComponent{
		Cfg: func() LoggerConfig {
			c := CommonFromConf(svc)
			return LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: svc}
		},
	}
}

// NewAdminComponent 构建标准 admin 组件：Enabled/IP/Port/Pprof 在 Start 时经
// CommonFromConf 读取（含 admin 端口为 0 时按服务类型回退，见 conf.ResolveAdminPort）；
// serviceName 由 runtime 默认取 app.Name()，无需重复标注。
//
// readyCheck 是运行期就绪探针：bus 服务传 router.ReadyCheck，bus 断连时 /readyz
// 返回 503 自动摘流；无探针的服务（websvr）传 nil。
func NewAdminComponent(app *runtime.App, readyCheck func() error) *runtime.AdminComponent {
	svc := mustAppName(app)
	opts := []runtime.AdminOption{
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := CommonFromConf(svc)
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
func NewTracingComponent(app *runtime.App) *TracingComponent {
	svc := mustAppName(app)
	return &TracingComponent{
		ServiceName: svc,
		Cfg:         func() ssrpc.TracingConfig { return CommonFromConf(svc).Tracing },
	}
}

// NewRouterComponent 构建标准 router/bus 组件：Common（self_bus_id、bus_mq_addr、
// register_addr）在 Start 时经 CommonFromConf 读取。
//
// transMgr 与 drivers 是服务相关参数，保持显式传入：drivers 必须显式装配（通常
// rabbitmq.NewRegistry()，只链接选定 driver），nil 时 Start 返回
// ErrDriversNotConfigured。onRecvSSPacket 可选，传入时覆盖默认的
// TransMgr.ProcessSSPacket 投递（connsvr 网关用于下行包短路回客户端）；至多取第一个。
func NewRouterComponent(app *runtime.App, transMgr transaction.ITransactionMgr, drivers *bus.DriverRegistry, onRecvSSPacket ...func(*sharedstruct.SSPacket)) *RouterComponent {
	svc := mustAppName(app)
	var onRecv func(*sharedstruct.SSPacket)
	if len(onRecvSSPacket) > 0 {
		onRecv = onRecvSSPacket[0]
	}
	return &RouterComponent{
		Common:         func() Common { return CommonFromConf(svc) },
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
