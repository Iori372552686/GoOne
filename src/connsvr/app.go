package connsvr

import (
	"context"
	"errors"

	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/service"
)

func NewApp() *bootstrap.ServiceApp {
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: "connsvr",
		LoadConfig: func() error {
			if err := gconf.LoadConnConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf: %+v", gconf.ConnSvrCfg)
			return nil
		},
		LoggerConfig: func() bootstrap.LoggerConfig {
			return bootstrap.LoggerConfig{
				Dir:   gconf.ConnSvrCfg.Debug.LogDir,
				Level: gconf.ConnSvrCfg.Debug.LogLevel,
				Name:  "connsvr",
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			return bootstrap.NewAdminConfig(
				"connsvr",
				misc.ServerType_ConnSvr,
				gconf.ConnSvrCfg.CommonRuntime.AdminServer.Enabled,
				gconf.ConnSvrCfg.CommonDebug.Pprof,
				gconf.ConnSvrCfg.CommonRuntime.AdminServer.IP,
				gconf.ConnSvrCfg.CommonRuntime.AdminServer.Port,
			)
		},
		InitDeps: func() error {
			if err := ssrpc.InitTracing("connsvr", ssrpc.TracingConfig{
				Enabled:      gconf.ConnSvrCfg.CommonRuntime.Tracing.Enabled,
				Exporter:     gconf.ConnSvrCfg.CommonRuntime.Tracing.Exporter,
				Endpoint:     gconf.ConnSvrCfg.CommonRuntime.Tracing.Endpoint,
				Insecure:     gconf.ConnSvrCfg.CommonRuntime.Tracing.Insecure,
				SamplerRatio: gconf.ConnSvrCfg.CommonRuntime.Tracing.SamplerRatio,
				Headers:      gconf.ConnSvrCfg.CommonRuntime.Tracing.Headers,
			}); err != nil {
				return err
			}
			globals.SignMgr.InitAndRun(gconf.ConnSvrCfg.Dependencies.HTTPSigns)
			globals.RestMgr.Init(gconf.ConnSvrCfg.Dependencies.RestApiConf, globals.SignMgr)
			return nil
		},
		RegisterHandlers: func() error {
			srv := connsvrv1.NewConnServiceSServer(&service.ConnServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			connsvrv1.RegisterConnServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
		StartRuntime: func() error {
			globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)
			if err := router.InitAndRun(
				gconf.ConnSvrCfg.Identity.SelfBusId,
				onRecvSSPacket,
				gconf.ConnSvrCfg.CommonRuntime.BusMQAddr,
				misc.ServerRouteRules,
				gconf.ConnSvrCfg.CommonRuntime.RegisterAddr,
			); err != nil {
				return err
			}
			if err := globals.ConnTcpSvr.CreateTcpServer("", gconf.ConnSvrCfg.Runtime.ListenPort+1, onTcpPacket); err != nil {
				return err
			}
			return globals.ConnWsSvr.CreateWebSocketServer("gin", "debug", gconf.ConnSvrCfg.Runtime.ListenPort, onWebSocketPacket)
		},
		OnProc: func() bool {
			return true
		},
		OnShutdown: func(ctx context.Context) error {
			router.BeginShutdown()
			shutdownErr := globals.TransMgr.Close(ctx)
			if err := router.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
			if err := ssrpc.ShutdownTracing(ctx); err != nil {
				shutdownErr = errors.Join(shutdownErr, err)
			}
			return shutdownErr
		},
		OnExit: func() {
			logger.Infof("================== connsvr Stop =========================")
		},
	})
}
