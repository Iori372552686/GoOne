package connsvr

import (
	"context"
	"fmt"

	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/misc"
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
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
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

	// 网关监听器：在 router/bus 起来之后启动 TCP/WS/KCP。当前三传输尚无 Stop/Drain
	// 能力（见 roadmap P0-07），这里作为 Start 阶段一次性拉起；Drain/Stop 留待
	// P0-08 后续补齐 listener 字段与 SessionTracker 接入。
	gateway := &bussvc.FuncComponent{
		ComponentName: "gateway_listeners",
		OnStart: func(_ context.Context) error {
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
		},
	}

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

	// Start 顺序：tracing → sign/rest → TransMgr → SSRPC 注册 → router/bus → 网关监听。
	for _, c := range []runtime.Component{tracing, signRestDeps, transMgr, registerHandlers, routerComp, gateway} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("connsvr register %s: %v", c.Name(), err))
		}
	}
	_ = misc.ServerType_ConnSvr
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
