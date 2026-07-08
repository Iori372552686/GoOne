package mysqlsvr

import (
	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap/busapp"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/service"
)

func NewApp() *bootstrap.ServiceApp {
	return busapp.New(busapp.Options{
		ServiceName: "mysqlsvr",
		ServerType:  misc.ServerType_MysqlSvr,
		LoadConfig: func() error {
			return gconf.LoadMySQLConfig(*gconf.SvrConfFile)
		},
		Common: func() busapp.Common {
			c := &gconf.MySqlSvrCfg
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
		InitDeps: func() error {
			return globals.OrmMgr.InitAndRun(gconf.MySqlSvrCfg.Dependencies.OrmConf, manager.GetTables()...)
		},
		RegisterHandlers: func() error {
			srv := mysqlsvrv1.NewMysqlServiceSServer(&service.MysqlServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			mysqlsvrv1.RegisterMysqlServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
		OnExit: func() {
			manager.Close()
			logger.Infof("================== mysqlsvr Stop =========================")
		},
	})
}
