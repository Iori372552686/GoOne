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
	"github.com/Iori372552686/GoOne/module/misc"
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

	app, err := runtime.New("mysqlsvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadMySQLConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for mysqlsvr")
			return nil
		}),
		runtime.WithDrainTimeout(30e9),
	)
	if err != nil {
		// New 仅在服务名非法时失败，此处服务名固定合法。
		panic(fmt.Sprintf("runtime.New mysqlsvr: %v", err))
	}

	// 注册顺序即 Start 顺序：tracing → orm 依赖 → TransMgr → SSRPC 注册 → router/bus。
	// 逆序用于 Quiesce/Drain/Stop。
	for _, c := range []runtime.Component{tracing, ormDeps, transMgr, registerHandlers, routerComp} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("mysqlsvr register %s: %v", c.Name(), err))
		}
	}
	_ = misc.ServerType_MysqlSvr // 保留 serverType 引用（admin 端口派生用，未来接入）。
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
