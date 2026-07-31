package roomcentersvr

import (
	"context"
	"time"

	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/service"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

// NewApp 用 runtime.App + Component 装配 roomcentersvr。
func NewApp() *runtime.App {
	transMgr := &bussvc.TransMgrComponent{
		Mgr: globals.TransMgr,
		Cfg: func() transaction.TransactionMgrConfig {
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
	}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(_ context.Context) error {
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
				// V4 P0-06：经 lib/contrib/config/factory 构造配置中心 client，
				// 由 gamedata.InitRemote 统一加载+热更；构造/拉取失败返回 error。
				if err := gamedata.InitNacos(gconf.RoomCenterSvrCfg.Dependencies.NacosConf); err != nil {
					return err
				}
			}
			return nil
		},
		// V4 P0-05：Stop 时关闭 Redis 连接池，消除连接泄漏。
		// V4 P0-06：同时取消 Nacos gamedata 监听，回收监听 goroutine。
		OnStop: func(_ context.Context) error {
			gamedata.StopNet()
			return rds.RedisMgr.Close()
		},
	}

	// P1-03：用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	// logger.RegisterCmdBacklist 是 roomcenter 特有的日志黑名单，作为独立 FuncComponent
	// 在 RegistryComponent 之后注册（注册顺序即 Start 顺序，故 TransMgr 绑定先于黑名单）。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := roomcenterv1.NewRoomCenterInnerServiceSServer(&service.RoomCenterInnerServiceImpl{}, ssrpc.DefaultMWOptions{})
			return roomcenterv1.RegisterRoomCenterInnerServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)
	cmdBacklist := &bussvc.FuncComponent{
		ComponentName: "cmd_backlist",
		OnStart: func(_ context.Context) error {
			logger.RegisterCmdBacklist(uint32(pb.CMD_ROOM_CENTER_INNER_TICK_REQ))
			return nil
		},
	}

	// P1-04：显式 DriverRegistry，只注册 rabbitmq。
	drivers := bus.NewDriverRegistry()
	drivers.MustRegister(rabbitmq.Driver())
	routerComp := &bussvc.RouterComponent{
		Common:   roomCommon,
		TransMgr: globals.TransMgr,
		Drivers:  drivers,
	}
	tracing := &bussvc.TracingComponent{
		ServiceName: "roomcentersvr",
		Cfg:         func() ssrpc.TracingConfig { return roomCommon().Tracing },
	}

	// 房间初始化：RoomListMgr.Init + 从 Redis 恢复房间快照 + AI 初始化房间。必须在
	// router 起来之后。
	roomInit := &bussvc.FuncComponent{
		ComponentName: "room_init",
		OnStart: func(_ context.Context) error {
			if err := globals.RoomListMgr.Init(); err != nil {
				return err
			}
			// 启动时从 Redis 恢复房间快照。
			globals.RoomListMgr.LoadAllFromDB()
			safego.Go(func() {
				room_ai.OnAiInitRoom()
			})
			return nil
		},
	}

	// 房间 Tick：原 OnTick 每 10ms 创建两个 goroutine（Tick + TickPersist）。替换为两
	// 个精确周期的 Task（Tick 5s、Persist 10s），NonOverlap 默认禁止重入，空闲时不
	// 再每 10ms 唤醒，也不再每秒创建约 200 个短命 goroutine。
	roomTick := scheduler.New("room_tick", 5*time.Second, func(_ context.Context) error {
		globals.RoomListMgr.Tick(time.Now().UnixMilli())
		return nil
	})
	roomPersist := scheduler.New("room_persist", 10*time.Second, func(_ context.Context) error {
		globals.RoomListMgr.TickPersist(time.Now().UnixMilli())
		return nil
	})

	// 房间落盘：原 OnExit。TransMgr 排空后全量落盘，避免数据丢失。作为 Drainer。
	roomFlush := &roomFlushComponent{}
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			c := roomCommon()
			return bussvc.LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: "roomcentersvr"}
		},
	}

	app := runtime.MustNew("roomcentersvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
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
		}),
	)

	// P0-03：admin 在 LoadConfig 后用 WithAdminConfig 延迟读取端口/IP，复用
	// app.tracker。
	adminComp := runtime.NewAdminComponent(app,
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := roomCommon()
			return runtime.AdminConfig{
				Enabled: c.AdminEnabled,
				IP:      c.AdminIP,
				Port:    c.AdminPort,
				Pprof:   c.Pprof,
			}
		}),
		runtime.WithAdminServiceName("roomcentersvr"),
		runtime.WithAdminReadyCheck(router.ReadyCheck),
	)

	// Start 顺序（P0-03）：datetime → logger → admin → tracing → 业务依赖 → SSRPC 注册
	// → TransMgr → router/bus → 房间初始化 → roomTick → roomPersist → roomFlush(Drainer)。
	// datetime_tick 放最前：room tick/房间初始化都依赖 datetime.NowMs()。
	// P1-07：用 MustRegister 一次注册全部组件。
	app.MustRegister(
		scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing,
		businessDeps, registerHandlers, cmdBacklist, transMgr, routerComp,
		roomInit, roomTick, roomPersist, roomFlush,
	)
	return app
}

// roomFlushComponent 在 Drain 阶段全量落盘所有房间（原 OnExit）。
type roomFlushComponent struct{}

func (roomFlushComponent) Name() string                  { return "room_flush" }
func (roomFlushComponent) Start(_ context.Context) error { return nil }

// Drain 实现 runtime.Drainer：TransMgr 已排空，强制全量写所有房间。
func (roomFlushComponent) Drain(_ context.Context) error {
	saved, failed := globals.RoomListMgr.FlushAllToDB()
	logger.Infof("roomcentersvr flush rooms on drain {saved:%d, failed:%d}", saved, failed)
	logger.Infof("================== roomcentersvr Stop =========================")
	return nil
}
func (roomFlushComponent) Stop(_ context.Context) error { return nil }

// roomCommon 从 gconf 产出 roomcentersvr 的 bus 服务共享配置段。
func roomCommon() bussvc.Common {
	c := &gconf.RoomCenterSvrCfg
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
