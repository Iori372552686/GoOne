package roomcentersvr

import (
	"context"
	"errors"

	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	service_router "github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/service"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

func NewApp() *bootstrap.ServiceApp {
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: "roomcentersvr",
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
		LoggerConfig: func() bootstrap.LoggerConfig {
			return bootstrap.LoggerConfig{
				Dir:   gconf.RoomCenterSvrCfg.Debug.LogDir,
				Level: gconf.RoomCenterSvrCfg.Debug.LogLevel,
				Name:  "roomcentersvr",
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			return bootstrap.NewAdminConfig(
				"roomcentersvr",
				misc.ServerType_RoomCenterSvr,
				gconf.RoomCenterSvrCfg.CommonRuntime.AdminServer.Enabled,
				gconf.RoomCenterSvrCfg.CommonDebug.Pprof,
				gconf.RoomCenterSvrCfg.CommonRuntime.AdminServer.IP,
				gconf.RoomCenterSvrCfg.CommonRuntime.AdminServer.Port,
			)
		},
		InitDeps: func() error {
			if err := ssrpc.InitTracing("roomcentersvr", ssrpc.TracingConfig{
				Enabled:      gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.Enabled,
				Exporter:     gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.Exporter,
				Endpoint:     gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.Endpoint,
				Insecure:     gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.Insecure,
				SamplerRatio: gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.SamplerRatio,
				Headers:      gconf.RoomCenterSvrCfg.CommonRuntime.Tracing.Headers,
			}); err != nil {
				return err
			}
			idGen, err := idgen.NewIDGen()
			if err != nil {
				return err
			}
			id.IDGen = idGen
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
		StartRuntime: func() error {
			transShardCount := gconf.RoomCenterSvrCfg.Capacity.TransShardCount
			if transShardCount <= 0 {
				transShardCount = transaction.DefaultShardCount()
			}
			globals.TransMgr.InitAndRunWithConfig(transaction.TransactionMgrConfig{
				MaxTrans:         misc.MaxTransNumber,
				ShardCount:       transShardCount,
				MaxPendingPerKey: 200,
			})
			logger.Infof("roomcentersvr transmgr shards=%d serial_key=routerid_or_uid", transShardCount)
			if err := service_router.InitAndRun(
				gconf.RoomCenterSvrCfg.Identity.SelfBusId,
				onRecvSSPacket,
				gconf.RoomCenterSvrCfg.CommonRuntime.BusMQAddr,
				misc.ServerRouteRules,
				gconf.RoomCenterSvrCfg.CommonRuntime.RegisterAddr,
			); err != nil {
				return err
			}
			if err := globals.RoomListMgr.Init(); err != nil {
				return err
			}
			safego.Go(func() {
				room_ai.OnAiInitRoom()
			})
			return nil
		},
		OnProc: func() bool {
			return true
		},
		OnTick: func(_, nowMs int64) {
			safego.Go(func() {
				globals.RoomListMgr.Tick(nowMs)
			})
		},
		OnShutdown: func(ctx context.Context) error {
			service_router.BeginShutdown()
			shutdownErr := globals.TransMgr.Close(ctx)
			if err := service_router.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
			if err := ssrpc.ShutdownTracing(ctx); err != nil {
				shutdownErr = errors.Join(shutdownErr, err)
			}
			return shutdownErr
		},
		OnExit: func() {
			logger.Infof("================== roomcentersvr Stop =========================")
		},
	})
}
