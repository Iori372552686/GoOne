package connsvr

import (
	"context"
	"fmt"

	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/service"
)

// NewApp 用 runtime.App + Component 装配 connsvr。
func NewApp() *runtime.App {
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	signRestDeps := &bussvc.FuncComponent{
		ComponentName: "sign_rest_deps",
		OnStart: func(_ context.Context) error {
			globals.SignMgr.InitAndRun(gconf.ConnSvrCfg.Dependencies.HTTPSigns)
			globals.RestMgr.Init(gconf.ConnSvrCfg.Dependencies.RestApiConf, globals.SignMgr)
			return nil
		},
	}

	registerHandlers := &bussvc.FuncComponent{
		ComponentName: "ssrpc_register",
		OnStart: func(_ context.Context) error {
			srv := connsvrv1.NewConnServiceSServer(&service.ConnServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			connsvrv1.RegisterConnServiceToDispatcher(d, srv)
			return d.RegisterToTransactionMgr(globals.TransMgr)
		},
	}

	// connsvr 特有：客户端下行包在 onRecvSSPacket 中短路回客户端，覆盖默认的
	// TransMgr.ProcessSSPacket。
	routerComp := &bussvc.RouterComponent{
		Common:         connCommon,
		TransMgr:       globals.TransMgr,
		OnRecvSSPacket: onRecvSSPacket,
	}
	tracing := &bussvc.TracingComponent{
		ServiceName: "connsvr",
		Cfg:         func() ssrpc.TracingConfig { return connCommon().Tracing },
	}
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			c := connCommon()
			return bussvc.LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: "connsvr"}
		},
	}

	// 网关监听器：在 router/bus 起来之后启动 TCP/WS/KCP。实现 Quiescer（停止接新连接）
	// 与 runtime.Component（Stop 强制关闭全部连接），满足 roadmap P0-07 网关排空。
	gateway := &gatewayComponent{}

	app, err := runtime.New("connsvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadConnConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for connsvr")
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("runtime.New connsvr: %v", err))
	}

	// P0-03：admin 在 LoadConfig 后用 WithAdminConfig 的 source 读取端口/IP（延迟取配
	// 置），直接复用 app.tracker（runtime.New 默认创建），无需外部传入。
	adminComp := runtime.NewAdminComponent(app,
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := connCommon()
			return runtime.AdminConfig{
				Enabled: c.AdminEnabled,
				IP:      c.AdminIP,
				Port:    c.AdminPort,
				Pprof:   c.Pprof,
			}
		}),
		runtime.WithAdminServiceName("connsvr"),
		runtime.WithAdminReadyCheck(router.ReadyCheck),
	)

	// Start 顺序（P0-03）：datetime_tick → logger → admin → tracing → dependencies →
	// ssrpc_registry → transaction_mgr → router → 网关。admin 紧跟 logger，使反向 Stop
	// 时 admin 在业务资源之后、logger 之前关闭；admin 在 LoadConfig 后 Start 故端口
	// 已生效。datetime_tick 放最前：tcp/ws/kcp 服务器启动期用 datetime.NowT() 设读写
	// deadline。
	for _, c := range []runtime.Component{scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing, signRestDeps, registerHandlers, transMgr, routerComp, gateway} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("connsvr register %s: %v", c.Name(), err))
		}
	}
	return app
}

// connCommon 从 gconf 产出 connsvr 的 bus 服务共享配置段。
func connCommon() bussvc.Common {
	c := &gconf.ConnSvrCfg
	return bussvc.Common{
		LogDir:       c.Debug.LogDir,
		LogLevel:     c.Debug.LogLevel,
		SelfBusId:    c.Identity.SelfBusId,
		BusMQAddr:    c.CommonRuntime.BusMQAddr,
		RegisterAddr: c.CommonRuntime.RegisterAddr,
		AdminEnabled: c.CommonRuntime.AdminServer.Enabled,
		AdminIP:      c.CommonRuntime.AdminServer.IP,
		AdminPort:    c.CommonRuntime.AdminServer.Port,
		Pprof:        c.CommonDebug.Pprof,
		Tracing: ssrpc.TracingConfig{
			Enabled:      c.CommonRuntime.Tracing.Enabled,
			Exporter:     c.CommonRuntime.Tracing.Exporter,
			Endpoint:     c.CommonRuntime.Tracing.Endpoint,
			Insecure:     c.CommonRuntime.Tracing.Insecure,
			SamplerRatio: c.CommonRuntime.Tracing.SamplerRatio,
			Headers:      c.CommonRuntime.Tracing.Headers,
		},
	}
}

// gatewayComponent 管理三传输（TCP/WS/KCP）网关监听器的启动、排空与停止（roadmap
// P0-07）。Start 拉起监听；Quiesce 停止接新连接但保留既有；Stop 强制关闭全部连接。
type gatewayComponent struct{}

func (gatewayComponent) Name() string { return "gateway_listeners" }

// Start 启动 TCP/WS/KCP 监听器。
func (gatewayComponent) Start(_ context.Context) error {
	if err := globals.ConnTcpSvr.CreateTcpServer(
		gconf.ConnSvrCfg.Runtime.TcpImplType,
		gconf.ConnSvrCfg.Runtime.ListenPort+1, onTcpPacket); err != nil {
		return err
	}
	if err := globals.ConnWsSvr.CreateWebSocketServer(
		"gin", "debug", gconf.ConnSvrCfg.Runtime.ListenPort, onWebSocketPacket); err != nil {
		return err
	}
	if kcpPort := gconf.ConnSvrCfg.Runtime.KcpPort; kcpPort > 0 {
		if err := globals.ConnKcpSvr.CreateKcpServer(kcpPort, onKcpPacket); err != nil {
			return err
		}
	}
	return nil
}

// Quiesce 实现 runtime.Quiescer：三传输停止接收新连接，SessionHub 拒绝新会话绑定
//（P0-05）。保留既有连接处理在途工作。readyz 此刻已返回 503（由状态机保证）。
func (gatewayComponent) Quiesce(_ context.Context) error {
	// P0-05：先让 hub 拒绝新 BindClient（draining），再停三传输 listener。
	globals.SessionHub.Quiesce()
	globals.ConnTcpSvr.Quiesce()
	globals.ConnWsSvr.Quiesce()
	globals.ConnKcpSvr.Quiesce()
	return nil
}

// Drain 实现 runtime.Drainer（P0-06）：等待逻辑会话（ActiveSessions）归零。
// 只等 session，不等未认证连接；超时由 App 的 drain 超时统一处理。
func (gatewayComponent) Drain(ctx context.Context) error {
	return globals.SessionTracker.WaitSessions(ctx)
}

// Stop 实现 runtime.Component：强制关闭三传输的全部残留连接，并关闭 SessionTracker
//（唤醒任何仍在等待的 Drain）。幂等。
func (gatewayComponent) Stop(_ context.Context) error {
	globals.ConnTcpSvr.Stop()
	globals.ConnWsSvr.Stop()
	globals.ConnKcpSvr.Stop()
	globals.SessionTracker.Close()
	return nil
}
