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
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap/busapp"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/service"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func NewApp() *bootstrap.ServiceApp {
	return busapp.New(busapp.Options{
		ServiceName: "mainsvr",
		ServerType:  misc.ServerType_MainSvr,
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
		Common: func() busapp.Common {
			c := &gconf.MainSvrCfg
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
		TransConfig: func() transaction.TransactionMgrConfig {
			transShardCount := gconf.MainSvrCfg.Capacity.TransShardCount
			if transShardCount <= 0 {
				transShardCount = transaction.DefaultShardCount()
			}
			logger.Infof("mainsvr transmgr shards=%d serial_key=routerid_or_uid", transShardCount)
			return transaction.TransactionMgrConfig{
				MaxTrans:         misc.MaxTransNumber,
				ShardCount:       transShardCount,
				MaxPendingPerKey: 100,
			}
		},
		InitDeps: func() error {
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
		StartExtra: func() error {
			// 心跳过期淘汰改经事务串行执行：Tick 协程只投递登出请求，
			// 落盘与删除在 Logout handler 内按 uid 串行键执行，
			// 消除 Tick 与业务 handler 对同一 *Role 的并发读写。
			role.SelfLogoutSender = func(uid uint64, zone uint32, req *g1_protocol.LogoutReq) {
				globals.TransMgr.SendPbMsgToMyself(router.SelfBusId(), uid, uid, zone, g1_protocol.CMD_MAIN_LOGOUT_REQ, req)
			}
			return nil
		},
		OnTick: func(lastMs, nowMs int64) {
			if lastMs/datetime.MS_PER_MINUTE != nowMs/datetime.MS_PER_MINUTE {
				safego.Go(func() { globals.RoleMgr.Tick() })
			}
		},
		OnShutdownExtra: func(ctx context.Context) error {
			// TransMgr 已排空，此时没有 handler 并发修改角色，安全地全量落盘，
			// 避免 write-behind 防抖窗口内的变更在停机时丢失。
			if _, failed := globals.RoleMgr.FlushAllToDB(); failed > 0 {
				return errors.New("failed to flush all roles to db on shutdown")
			}
			return nil
		},
		OnExit: func() {
			logger.Infof("================== mainsvr Stop =========================")
		},
	})
}
