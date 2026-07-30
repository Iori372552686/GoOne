package mysqlsvr

import (
	"context"

	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/gconf"
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
	// P1-03：用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := mysqlsvrv1.NewMysqlServiceSServer(&service.MysqlServiceImpl{}, ssrpc.DefaultMWOptions{})
			return mysqlsvrv1.RegisterMysqlServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// ORM 依赖：Start 初始化（启动异步 worker + 初始化 ORM Engine）；Stop 关闭。
	// V3-P0-05：worker 启动从 package init 移到显式 manager.Start，使 import 不再
	// 产生 goroutine，生命周期由 Component 统一管理。
	ormDeps := &bussvc.FuncComponent{
		ComponentName: "orm_deps",
		OnStart: func(_ context.Context) error {
			manager.Start()
			return globals.OrmMgr.InitAndRun(gconf.MySqlSvrCfg.Dependencies.OrmConf, manager.GetTables()...)
		},
		OnStop: func(_ context.Context) error {
			manager.Close()
			logger.Infof("================== mysqlsvr Stop =========================")
			return nil
		},
	}

	// P1-04：显式 DriverRegistry，只注册 rabbitmq。
	drivers := bus.NewDriverRegistry()
	drivers.MustRegister(rabbitmq.Driver())
	routerComp := &bussvc.RouterComponent{
		Common:   mysqlCommon,
		TransMgr: globals.TransMgr,
		Drivers:  drivers,
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

	app := runtime.MustNew("mysqlsvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadMySQLConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for mysqlsvr")
			return nil
		}),
	)

	// P0-03：admin 在 LoadConfig 后用 WithAdminConfig 延迟读取端口/IP，复用
	// app.tracker。
	adminComp := runtime.NewAdminComponent(app,
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := mysqlCommon()
			return runtime.AdminConfig{
				Enabled: c.AdminEnabled,
				IP:      c.AdminIP,
				Port:    c.AdminPort,
				Pprof:   c.Pprof,
			}
		}),
		runtime.WithAdminServiceName("mysqlsvr"),
		runtime.WithAdminReadyCheck(router.ReadyCheck),
	)

	// 注册顺序即 Start 顺序（P0-03）：datetime 周期刷新 → logger → admin → tracing →
	// orm 依赖 → SSRPC 注册（必须在 TransMgr.InitAndRun 之前，因
	// RegisterToTransactionMgr 调 RegisterCmd）→ TransMgr → router/bus。
	// P1-07：用 MustRegister 一次注册全部组件。
	app.MustRegister(
		scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing,
		ormDeps, registerHandlers, transMgr, routerComp,
	)
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
