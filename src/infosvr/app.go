package infosvr

import (
	"context"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	"github.com/Iori372552686/GoOne/src/infosvr/service"
)

// NewApp 用 runtime.App + Component 装配 infosvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	app := runtime.MustNew("infosvr", bussvc.WithConfLoader())

	// 标准组件：服务名取 app.Name()，Start 时（LoadConfig 之后）自读 conf。
	logComp := bussvc.NewLoggerComponent(app)
	adminComp := bussvc.NewAdminComponent(app, router.ReadyCheck)
	tracing := bussvc.NewTracingComponent(app)
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	redisDeps := &bussvc.FuncComponent{
		ComponentName: "redis_deps",
		OnStart: func(_ context.Context) error {
			var dbs []redis.Config
			if err := conf.Unmarshal("base_cfg.dependencies.db_instances", &dbs); err != nil {
				return err
			}
			return globals.InfoMgr.RedisMgr.InitAndRun(dbs)
		},
		// Stop 时关闭 Redis 连接池，消除连接泄漏。
		OnStop: func(_ context.Context) error {
			return globals.InfoMgr.RedisMgr.Close()
		},
	}

	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := infosvrv1.NewInfoServiceSServer(&service.InfoServiceImpl{}, ssrpc.DefaultMWOptions{})
			return infosvrv1.RegisterInfoServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	// Start 顺序：datetime_tick → logger → admin → tracing → dependencies →
	// ssrpc_registry → transaction_mgr → router。datetime_tick 放最前。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(
		scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing,
		redisDeps, registerHandlers, transMgr, routerComp,
	)
	return app
}
