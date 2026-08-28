package roomcentersvr

import (
	"context"
	"time"

	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/module/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	id "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/idgen"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/service"
	pb "github.com/Iori372552686/g1_common/protocol"
)

// NewApp 用 runtime.App + Component 装配 roomcentersvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	// gamedata 本地目录加载作为 LoadConfig 的追加钩子，在校验通过后执行。
	app := bussvc.MustNew("roomcentersvr", router.ReadyCheck, bussvc.WithConfLoader(func(_ context.Context) error {
		if gameDataDir := conf.Get("base_cfg.dependencies.game_data_dir").String(); gameDataDir != "" {
			logger.Infof("Loading local file by gameconf_dir: %v ", gameDataDir)
			if err := gamedata.InitLocal(gameDataDir); err != nil {
				return err
			}
		}
		return nil
	}))

	// 标准组件（datetime/logger/admin/tracing）由 bussvc.MustNew 集中注册。

	// TransMgr：房间高频同键包（tick/操作）需要更大的每键排队上限，显式给 200；
	// 其余（分片数/MaxTrans）用组件内部默认。
	transMgr := &bussvc.TransMgrComponent{
		Mgr: globals.TransMgr,
		Cfg: transaction.TransactionMgrConfig{MaxPendingPerKey: 200},
	}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(_ context.Context) error {
			idGen, err := idgen.NewIDGen()
			if err != nil {
				return err
			}
			id.IDGen = idGen
			// 初始化 Redis（房间快照持久化用）。OnStart 内部对空配置静默跳过，
			// 保留 "redis 可选" 的向后兼容语义。
			if err := rds.RedisMgr.OnStart(nil); err != nil {
				return err
			}
			var nacosConf net_conf.NacosConf
			_ = conf.Unmarshal("base_cfg.dependencies.nacos_conf", &nacosConf)
			if nacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", nacosConf.GroupName)
				// 经 lib/contrib/config/factory 构造配置中心 client，
				// 由 gamedata.InitRemote 统一加载+热更；构造/拉取失败返回 error。
				if err := gamedata.InitNacos(nacosConf); err != nil {
					return err
				}
			}
			return nil
		},
		// Stop 时关闭 Redis 连接池，消除连接泄漏。
		// 同时取消 Nacos gamedata 监听，回收监听 goroutine。
		OnStop: func(_ context.Context) error {
			gamedata.StopNet()
			return rds.RedisMgr.Close()
		},
	}

	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
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

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	// 房间初始化：RoomListMgr.Init + AI 初始化房间。必须在 router 起来之后。
	// 房间快照恢复不再在启动时全量执行（旧 LoadAllFromDB 因 zone/stage 懒创建而
	// 恒为空转），改为 GetTexasObj 首次创建 stage 时在临界区内懒恢复（见
	// texas_room/data_proc.go），精确恢复本实例负责的分片。
	roomInit := &bussvc.FuncComponent{
		ComponentName: "room_init",
		OnStart: func(_ context.Context) error {
			if err := globals.RoomListMgr.Init(); err != nil {
				return err
			}
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

	// 房间落盘：原 OnExit。作为 Drainer 全量落盘所有房间。
	//
	// 注册顺序说明（重要，Drain 顺序修复）：
	// runtime 对 Quiesce/Drain/Stop 一律按注册序的【逆序】执行（run.go drainComponents）。
	// 因此要让 roomFlush 在 TransMgr 排空【之后】执行（此时已无 handler 并发修改房间），
	// 必须把它注册在 transMgr【之前】。旧实现注册在最后，导致 Flush 与在途事务并发，
	// 存在停机丢数据/数据竞争风险。
	roomFlush := &roomFlushComponent{}

	// Start 顺序：datetime（WithConfLoader 自动注册，隐含在最前）→ logger → admin
	// → tracing → 业务依赖 → SSRPC 注册 → roomFlush(Start 空操作) → TransMgr
	// → router/bus → 房间初始化 → roomTick → roomPersist。
	// Drain 逆序：roomPersist → roomTick → roomInit → routerComp → transMgr → roomFlush
	// —— TransMgr 排空后才全量落盘。datetime_tick 必须最前：room tick/房间初始化
	// 都依赖 datetime.NowMs()，由 WithConfLoader 自动注册保证。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(
		businessDeps, registerHandlers, cmdBacklist, roomFlush, transMgr, routerComp,
		roomInit, roomTick, roomPersist,
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
