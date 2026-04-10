package infosvr

import (
	"context"
	"errors"
	"fmt"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	"github.com/Iori372552686/GoOne/src/infosvr/service"
)

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

func NewApp() *bootstrap.ServiceApp {
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: "infosvr",
		LoadConfig: func() error {
			return gconf.LoadInfoConfig(*gconf.SvrConfFile)
		},
		LoggerConfig: func() bootstrap.LoggerConfig {
			return bootstrap.LoggerConfig{
				Dir:   gconf.InfoSvrCfg.Debug.LogDir,
				Level: gconf.InfoSvrCfg.Debug.LogLevel,
				Name:  "infosvr",
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			return bootstrap.NewAdminConfig(
				"infosvr",
				misc.ServerType_InfoSvr,
				gconf.InfoSvrCfg.CommonRuntime.AdminServer.Enabled,
				gconf.InfoSvrCfg.CommonDebug.Pprof,
				gconf.InfoSvrCfg.CommonRuntime.AdminServer.IP,
				gconf.InfoSvrCfg.CommonRuntime.AdminServer.Port,
			)
		},
		ComponentStatuses: func() []bootstrap.ComponentStatus {
			return buildInfoSvrComponentStatuses(
				globals.InfoMgr.RedisMgr.InstanceCount(),
				globals.TransMgr.StatsSnapshot(),
				router.Snapshot(),
			)
		},
		InitDeps: func() error {
			if err := ssrpc.InitTracing("infosvr", ssrpc.TracingConfig{
				Enabled:      gconf.InfoSvrCfg.CommonRuntime.Tracing.Enabled,
				Exporter:     gconf.InfoSvrCfg.CommonRuntime.Tracing.Exporter,
				Endpoint:     gconf.InfoSvrCfg.CommonRuntime.Tracing.Endpoint,
				Insecure:     gconf.InfoSvrCfg.CommonRuntime.Tracing.Insecure,
				SamplerRatio: gconf.InfoSvrCfg.CommonRuntime.Tracing.SamplerRatio,
				Headers:      gconf.InfoSvrCfg.CommonRuntime.Tracing.Headers,
			}); err != nil {
				return err
			}
			return globals.InfoMgr.RedisMgr.InitAndRun(gconf.InfoSvrCfg.Dependencies.DbInstances)
		},
		RegisterHandlers: func() error {
			srv := infosvrv1.NewInfoServiceSServer(&service.InfoServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			infosvrv1.RegisterInfoServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
		StartRuntime: func() error {
			globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)
			return router.InitAndRun(
				gconf.InfoSvrCfg.Identity.SelfBusId,
				onRecvSSPacket,
				gconf.InfoSvrCfg.CommonRuntime.BusMQAddr,
				misc.ServerRouteRules,
				gconf.InfoSvrCfg.CommonRuntime.RegisterAddr,
			)
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
			logger.Infof("================== infosvr Stop =========================")
		},
	})
}

func buildInfoSvrComponentStatuses(redisInstances int, txStats transaction.TransactionMgrStats, routerSnapshot router.AdminSnapshot) []bootstrap.ComponentStatus {
	redisStatus := bootstrap.ComponentStatus{
		Name:    "infosvr.redis",
		State:   "pending",
		Ready:   false,
		Message: "waiting for redis initialization",
	}
	if redisInstances > 0 {
		redisStatus.State = "ready"
		redisStatus.Ready = true
		redisStatus.Message = fmt.Sprintf("redis instances=%d", redisInstances)
	}

	transactionStatus := bootstrap.ComponentStatus{
		Name:    "infosvr.transaction_mgr",
		State:   "pending",
		Ready:   false,
		Message: fmt.Sprintf("shards=%d active=%d pending=%d dropped=%d", txStats.ShardCount, txStats.ActiveTransactions, txStats.PendingPackets, txStats.DroppedPackets),
	}
	if txStats.ShardCount > 0 {
		transactionStatus.State = "ready"
		transactionStatus.Ready = true
	}

	routerStatus := bootstrap.ComponentStatus{
		Name:    "infosvr.router",
		State:   "pending",
		Ready:   false,
		Message: "router not initialized",
	}
	if routerSnapshot.Initialized && routerSnapshot.SelfBusID != 0 {
		routerStatus.State = "ready"
		routerStatus.Ready = !routerSnapshot.ShuttingDown
		routerStatus.Message = fmt.Sprintf("bus_id=%d shutting_down=%t", routerSnapshot.SelfBusID, routerSnapshot.ShuttingDown)
	}

	return []bootstrap.ComponentStatus{
		redisStatus,
		transactionStatus,
		routerStatus,
	}
}
