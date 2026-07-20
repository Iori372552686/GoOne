package infosvr

import (
	"context"
	"fmt"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	"github.com/Iori372552686/GoOne/src/infosvr/service"
)

// NewApp 用 runtime.App + Component 装配 infosvr。
func NewApp() *runtime.App {
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	redisDeps := &bussvc.FuncComponent{
		ComponentName: "redis_deps",
		OnStart: func(_ context.Context) error {
			return globals.InfoMgr.RedisMgr.InitAndRun(gconf.InfoSvrCfg.Dependencies.DbInstances)
		},
	}

	registerHandlers := &bussvc.FuncComponent{
		ComponentName: "ssrpc_register",
		OnStart: func(_ context.Context) error {
			srv := infosvrv1.NewInfoServiceSServer(&service.InfoServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			infosvrv1.RegisterInfoServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
	}

	routerComp := &bussvc.RouterComponent{
		Common:   infoCommon,
		TransMgr: globals.TransMgr,
	}
	tracing := &bussvc.TracingComponent{
		ServiceName: "infosvr",
		Cfg:         func() ssrpc.TracingConfig { return infoCommon().Tracing },
	}
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			c := infoCommon()
			return bussvc.LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: "infosvr"}
		},
	}

	app, err := runtime.New("infosvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadInfoConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for infosvr")
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("runtime.New infosvr: %v", err))
	}

	ic := infoCommon()
	tracker := runtime.NewComponentTracker(nil)
	adminComp := runtime.NewAdminComponent(app, tracker,
		runtime.WithAdminListen(ic.AdminIP, ic.AdminPort),
		runtime.WithAdminPprof(ic.Pprof),
		runtime.WithAdminServiceName("infosvr"),
		runtime.WithAdminReadyCheck(router.ReadyCheck),
	)

	// datetime_tick 放最前：redis/tracing 启动期可能读 datetime。
	for _, c := range []runtime.Component{scheduler.DefaultDateTimeTick(), logComp, tracing, redisDeps, registerHandlers, transMgr, routerComp, adminComp} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("infosvr register %s: %v", c.Name(), err))
		}
	}
	return app
}

// infoCommon 从 gconf 产出 infosvr 的 bus 服务共享配置段。
func infoCommon() bussvc.Common {
	c := &gconf.InfoSvrCfg
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

// buildInfoSvrComponentStatuses 聚合 infosvr 的自定义组件状态（redis/transaction/router），
// 供未来 admin /components 端点接入使用。
func buildInfoSvrComponentStatuses(redisInstances int, txStats transaction.TransactionMgrStats, routerSnapshot router.AdminSnapshot) []runtime.ComponentReport {
	redisStatus := runtime.ComponentReport{Name: "infosvr.redis", State: "pending"}
	if redisInstances > 0 {
		redisStatus.State = "running"
		redisStatus.Ready = true
		redisStatus.Message = fmt.Sprintf("redis instances=%d", redisInstances)
	}
	transactionStatus := runtime.ComponentReport{
		Name:    "infosvr.transaction_mgr",
		State:   "pending",
		Message: fmt.Sprintf("shards=%d active=%d pending=%d dropped=%d", txStats.ShardCount, txStats.ActiveTransactions, txStats.PendingPackets, txStats.DroppedPackets),
	}
	if txStats.ShardCount > 0 {
		transactionStatus.State = "running"
		transactionStatus.Ready = true
	}
	routerStatus := runtime.ComponentReport{Name: "infosvr.router", State: "pending", Message: "router not initialized"}
	if routerSnapshot.Initialized && routerSnapshot.SelfBusID != 0 {
		routerStatus.State = "running"
		routerStatus.Ready = !routerSnapshot.ShuttingDown
		routerStatus.Message = fmt.Sprintf("bus_id=%d shutting_down=%t", routerSnapshot.SelfBusID, routerSnapshot.ShuttingDown)
	}
	return []runtime.ComponentReport{redisStatus, transactionStatus, routerStatus}
}
