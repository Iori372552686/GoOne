package mainsvr

import (
	"context"
	"errors"

	mainsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mainsvr/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/service"
)

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

func NewApp() *bootstrap.ServiceApp {
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: "mainsvr",
		LoadConfig: func() error {
			if err := gconf.LoadMainConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			if gconf.MainSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.MainSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.MainSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			logger.Infof("gconf file load success | %+v", gconf.MainSvrCfg)
			return nil
		},
		LoggerConfig: func() bootstrap.LoggerConfig {
			return bootstrap.LoggerConfig{
				Dir:   gconf.MainSvrCfg.Debug.LogDir,
				Level: gconf.MainSvrCfg.Debug.LogLevel,
				Name:  "mainsvr",
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			return bootstrap.NewAdminConfig(
				"mainsvr",
				misc.ServerType_MainSvr,
				gconf.MainSvrCfg.CommonRuntime.AdminServer.Enabled,
				gconf.MainSvrCfg.CommonDebug.Pprof,
				gconf.MainSvrCfg.CommonRuntime.AdminServer.IP,
				gconf.MainSvrCfg.CommonRuntime.AdminServer.Port,
			)
		},
		InitDeps: func() error {
			if err := ssrpc.InitTracing("mainsvr", ssrpc.TracingConfig{
				Enabled:      gconf.MainSvrCfg.CommonRuntime.Tracing.Enabled,
				Exporter:     gconf.MainSvrCfg.CommonRuntime.Tracing.Exporter,
				Endpoint:     gconf.MainSvrCfg.CommonRuntime.Tracing.Endpoint,
				Insecure:     gconf.MainSvrCfg.CommonRuntime.Tracing.Insecure,
				SamplerRatio: gconf.MainSvrCfg.CommonRuntime.Tracing.SamplerRatio,
				Headers:      gconf.MainSvrCfg.CommonRuntime.Tracing.Headers,
			}); err != nil {
				return err
			}
			sensitive_words.Init(gconf.MainSvrCfg.Dependencies.SensitiveWordsFile)
			if err := rds.RedisMgr.InitAndRun(gconf.MainSvrCfg.Dependencies.DbInstances); err != nil {
				return err
			}
			idGen, err := idgen.NewIDGen()
			if err != nil {
				return err
			}
			globals.IDGen = idGen
			if gconf.MainSvrCfg.Dependencies.NacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", gconf.MainSvrCfg.Dependencies.NacosConf.GroupName)
				if err := gamedata.InitNet(net_conf.NewNacosConfigClient(gconf.MainSvrCfg.Dependencies.NacosConf), gconf.MainSvrCfg.Dependencies.NacosConf.GroupName); err != nil {
					return err
				}
			}
			return nil
		},
		RegisterHandlers: func() error {
			srv := mainsvrv1.NewMainC2SServiceSServer(&service.MainC2SServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			mainsvrv1.RegisterMainC2SServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
		StartRuntime: func() error {
			transShardCount := gconf.MainSvrCfg.Capacity.TransShardCount
			if transShardCount <= 0 {
				transShardCount = transaction.DefaultShardCount()
			}
			globals.TransMgr.InitAndRunWithConfig(transaction.TransactionMgrConfig{
				MaxTrans:         misc.MaxTransNumber,
				ShardCount:       transShardCount,
				MaxPendingPerKey: 100,
			})
			logger.Infof("mainsvr transmgr shards=%d serial_key=routerid_or_uid", transShardCount)
			return router.InitAndRun(
				gconf.MainSvrCfg.Identity.SelfBusId,
				onRecvSSPacket,
				gconf.MainSvrCfg.CommonRuntime.BusMQAddr,
				misc.ServerRouteRules,
				gconf.MainSvrCfg.CommonRuntime.RegisterAddr,
			)
		},
		OnProc: func() bool {
			return true
		},
		OnTick: func(lastMs, nowMs int64) {
			if lastMs/datetime.MS_PER_MINUTE != nowMs/datetime.MS_PER_MINUTE {
				safego.Go(func() { globals.RoleMgr.Tick() })
			}
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
			logger.Infof("================== mainsvr Stop =========================")
		},
	})
}
