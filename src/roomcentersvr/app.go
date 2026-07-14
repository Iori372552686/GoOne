package roomcentersvr

import (
	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap/busapp"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/service"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

func NewApp() *bootstrap.ServiceApp {
	return busapp.New(busapp.Options{
		ServiceName: "roomcentersvr",
		ServerType:  misc.ServerType_RoomCenterSvr,
		LoadConfig: func() error {
			if err := gconf.LoadRoomCenterConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			if gconf.RoomCenterSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.RoomCenterSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.RoomCenterSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			return nil
		},
		Common: func() busapp.Common {
			c := &gconf.RoomCenterSvrCfg
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
			transShardCount := gconf.RoomCenterSvrCfg.Capacity.TransShardCount
			if transShardCount <= 0 {
				transShardCount = transaction.DefaultShardCount()
			}
			logger.Infof("roomcentersvr transmgr shards=%d serial_key=routerid_or_uid", transShardCount)
			return transaction.TransactionMgrConfig{
				MaxTrans:         misc.MaxTransNumber,
				ShardCount:       transShardCount,
				MaxPendingPerKey: 200,
			}
		},
		InitDeps: func() error {
			idGen, err := idgen.NewIDGen()
			if err != nil {
				return err
			}
			id.IDGen = idGen
			// 初始化 Redis（房间快照持久化用）。无配置时跳过，保持向后兼容。
			if len(gconf.RoomCenterSvrCfg.Dependencies.DbInstances) > 0 {
				if err := rds.RedisMgr.InitAndRun(gconf.RoomCenterSvrCfg.Dependencies.DbInstances); err != nil {
					return err
				}
				logger.Infof("roomcentersvr redis initialized for room snapshot persistence")
			}
			if gconf.RoomCenterSvrCfg.Dependencies.NacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", gconf.RoomCenterSvrCfg.Dependencies.NacosConf.GroupName)
				if err := gamedata.InitNet(net_conf.NewNacosConfigClient(gconf.RoomCenterSvrCfg.Dependencies.NacosConf), gconf.RoomCenterSvrCfg.Dependencies.NacosConf.GroupName); err != nil {
					return err
				}
			}
			return nil
		},
		RegisterHandlers: func() error {
			srv := roomcenterv1.NewRoomCenterInnerServiceSServer(&service.RoomCenterInnerServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			roomcenterv1.RegisterRoomCenterInnerServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			logger.RegisterCmdBacklist(uint32(pb.CMD_ROOM_CENTER_INNER_TICK_REQ))
			return nil
		},
		StartExtra: func() error {
			if err := globals.RoomListMgr.Init(); err != nil {
				return err
			}
			// 启动时从 Redis 恢复房间快照（任务 2.5）。
			globals.RoomListMgr.LoadAllFromDB()
			safego.Go(func() {
				room_ai.OnAiInitRoom()
			})
			return nil
		},
		OnTick: func(_, nowMs int64) {
			safego.Go(func() {
				globals.RoomListMgr.Tick(nowMs)
			})
			// 周期持久化变更的房间快照（与 tick 节流分离，独立节拍）。
			safego.Go(func() {
				globals.RoomListMgr.TickPersist(nowMs)
			})
		},
		OnExit: func() {
			logger.Infof("================== roomcentersvr Stop =========================")
			// 停机前强制全量写所有房间，避免数据丢失。
			saved, failed := globals.RoomListMgr.FlushAllToDB()
			logger.Infof("roomcentersvr flush rooms on exit {saved:%d, failed:%d}", saved, failed)
		},
	})
}
