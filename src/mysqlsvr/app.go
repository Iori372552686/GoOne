package mysqlsvr

import (
	"context"

	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/service"
)

// NewApp 用 runtime.App + Component 装配 mysqlsvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	app := runtime.MustNew("mysqlsvr", bussvc.WithConfLoader())

	// 标准组件：服务名取 app.Name()，Start 时（LoadConfig 之后）自读 conf。
	// logger 最早启动、最晚停止（Stop 时 Flush 落盘）。
	logComp := bussvc.NewLoggerComponent(app)
	adminComp := bussvc.NewAdminComponent(app, router.ReadyCheck)
	tracing := bussvc.NewTracingComponent(app)

	// TransMgr：Start 启动分片 worker；Drain 排空在途事务（受 ctx 超时约束）。
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	// SSRPC 注册：必须在 Seal（于 router/bus Start 之前）完成，且在 TransMgr Start
	// 之后（RegisterToTransactionMgr 依赖已 InitAndRun 的 TransMgr）。这里作为
	// FuncComponent，注册顺序置于 TransMgr 之后、Router 之前。
	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := mysqlsvrv1.NewMysqlServiceSServer(&service.MysqlServiceImpl{}, ssrpc.DefaultMWOptions{})
			return mysqlsvrv1.RegisterMysqlServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// ORM 依赖：Start 初始化（启动异步 worker + 初始化 ORM Engine）；Stop 关闭。
	// worker 启动从 package init 移到显式 manager.Start，使 import 不再
	// 产生 goroutine，生命周期由 Component 统一管理。
	ormDeps := &bussvc.FuncComponent{
		ComponentName: "orm_deps",
		OnStart: func(_ context.Context) error {
			manager.Start()
			var ormConf []orm.Config
			if err := conf.Unmarshal("base_cfg.dependencies.orm_instances", &ormConf); err != nil {
				return err
			}
			return globals.OrmMgr.InitAndRun(ormConf, manager.GetTables()...)
		},
		OnStop: func(_ context.Context) error {
			manager.Close()
			// 同时关闭 ORM Engine（此前只关 async worker，Engine 靠 OS 回收）。
			_ = globals.OrmMgr.Close()
			logger.Infof("================== mysqlsvr Stop =========================")
			return nil
		},
	}

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	// 注册顺序即 Start 顺序：datetime 周期刷新 → logger → admin → tracing →
	// orm 依赖 → SSRPC 注册（必须在 TransMgr.InitAndRun 之前，因
	// RegisterToTransactionMgr 调 RegisterCmd）→ TransMgr → router/bus。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(
		scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing,
		ormDeps, registerHandlers, transMgr, routerComp,
	)
	return app
}
