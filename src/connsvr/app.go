package connsvr

import (
	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap/busapp"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/service"
)

func NewApp() *bootstrap.ServiceApp {
	return busapp.New(busapp.Options{
		ServiceName: "connsvr",
		ServerType:  misc.ServerType_ConnSvr,
		LoadConfig: func() error {
			if err := gconf.LoadConnConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf: %+v", gconf.ConnSvrCfg)
			return nil
		},
		Common: func() busapp.Common {
			c := &gconf.ConnSvrCfg
			return busapp.Common{
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
		},
		TransMgr: globals.TransMgr,
		// connsvr 特有：客户端下行包在 onRecvSSPacket 中直接短路回客户端。
		OnRecvSSPacket: onRecvSSPacket,
		InitDeps: func() error {
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
		StartExtra: func() error {
			if err := globals.ConnTcpSvr.CreateTcpServer("", gconf.ConnSvrCfg.Runtime.ListenPort+1, onTcpPacket); err != nil {
				return err
			}
			return globals.ConnWsSvr.CreateWebSocketServer("gin", "debug", gconf.ConnSvrCfg.Runtime.ListenPort, onWebSocketPacket)
		},
		OnExit: func() {
			logger.Infof("================== connsvr Stop =========================")
		},
	})
}
