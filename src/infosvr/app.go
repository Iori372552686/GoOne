package infosvr

import (
	"fmt"

	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap/busapp"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	"github.com/Iori372552686/GoOne/src/infosvr/service"
)

func NewApp() *bootstrap.ServiceApp {
	return busapp.New(busapp.Options{
		ServiceName: "infosvr",
		ServerType:  misc.ServerType_InfoSvr,
		LoadConfig: func() error {
			return gconf.LoadInfoConfig(*gconf.SvrConfFile)
		},
		Common: func() busapp.Common {
			c := &gconf.InfoSvrCfg
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
		ComponentStatuses: func() []bootstrap.ComponentStatus {
			return buildInfoSvrComponentStatuses(
				globals.InfoMgr.RedisMgr.InstanceCount(),
				globals.TransMgr.StatsSnapshot(),
				router.Snapshot(),
			)
		},
		InitDeps: func() error {
			return globals.InfoMgr.RedisMgr.InitAndRun(gconf.InfoSvrCfg.Dependencies.DbInstances)
		},
		RegisterHandlers: func() error {
			srv := infosvrv1.NewInfoServiceSServer(&service.InfoServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			infosvrv1.RegisterInfoServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
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
