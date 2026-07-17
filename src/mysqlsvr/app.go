package mysqlsvr

import (
	"context"
	"fmt"

	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/service"
)

// NewApp 用 runtime.App + Component 装配 mysqlsvr，取代旧的
// application.Init/Run + bootstrap.ServiceApp Hook 生命周期。
func NewApp() *runtime.App {
	// TransMgr：Start 启动分片 worker；Drain 排空在途事务（受 ctx 超时约束）。
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	// SSRPC 注册：必须在 Seal（于 router/bus Start 之前）完成，且在 TransMgr Start
	// 之后（RegisterToTransactionMgr 依赖已 InitAndRun 的 TransMgr）。这里作为
	// FuncComponent，注册顺序置于 TransMgr 之后、Router 之前。
	registerHandlers := &bussvc.FuncComponent{
		ComponentName: "ssrpc_register",
		OnStart: func(_ context.Context) error {
			srv := mysqlsvrv1.NewMysqlServiceSServer(&service.MysqlServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			mysqlsvrv1.RegisterMysqlServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
	}

	// ORM 依赖：Start 初始化；Stop 关闭（原 OnExit 的 manager.Close）。
	ormDeps := &bussvc.FuncComponent{
		ComponentName: "orm_deps",
		OnStart: func(_ context.Context) error {
			return globals.OrmMgr.InitAndRun(gconf.MySqlSvrCfg.Dependencies.OrmConf, manager.GetTables()...)
		},
		OnStop: func(_ context.Context) error {
			manager.Close()
			logger.Infof("================== mysqlsvr Stop =========================")
			return nil
		},
	}

	routerComp := &bussvc.RouterComponent{
		Common:   mysqlCommon,
		TransMgr: globals.TransMgr,
	}
	tracing := &bussvc.TracingComponent{
		ServiceName: "mysqlsvr",
		Cfg:         func() ssrpc.TracingConfig { return mysqlCommon().Tracing },
	}
	// logger：最早启动、最晚停止（Stop 时 Flush 落盘）。
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			c := mysqlCommon()
			return bussvc.LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: "mysqlsvr"}
		},
	}

	app, err := runtime.New("mysqlsvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadMySQLConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for mysqlsvr")
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("runtime.New mysqlsvr: %v", err))
	}

	// admin 需引用 app（healthz/readyz 读取 app.State()），故在 app 创建后构造。
	c := mysqlCommon()
	tracker := runtime.NewComponentTracker(nil)
	adminComp := runtime.NewAdminComponent(app, tracker,
		runtime.WithAdminListen(c.AdminIP, c.AdminPort),
		runtime.WithAdminPprof(c.Pprof),
		runtime.WithAdminServiceName("mysqlsvr"),
	)

	// 注册顺序即 Start 顺序：logger → tracing → orm 依赖 → TransMgr → SSRPC 注册 →
	// router/bus → admin。逆序用于 Quiesce/Drain/Stop。
	for _, comp := range []runtime.Component{logComp, tracing, ormDeps, transMgr, registerHandlers, routerComp, adminComp} {
		if err := app.Register(comp); err != nil {
			panic(fmt.Sprintf("mysqlsvr register %s: %v", comp.Name(), err))
		}
	}
	return app
}

// mysqlCommon 从 gconf 产出 bus 服务共享配置段。
func mysqlCommon() bussvc.Common {
	c := &gconf.MySqlSvrCfg
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
