package mainsvr

import (
	"context"
	"errors"
	"time"

	mainsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mainsvr/v1"
	"github.com/Iori372552686/GoOne/module/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/service"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// NewApp 用 runtime.App + Component 装配 mainsvr。服务名只在 MustNew 出现一次；
func NewApp() *runtime.App {
	// gamedata 本地目录加载作为 LoadConfig 的追加钩子，在校验通过后执行。
	app := bussvc.MustNew("mainsvr", router.ReadyCheck, bussvc.WithConfLoader(func(_ context.Context) error {
		if gameDataDir := conf.Get("base_cfg.dependencies.game_data_dir").String(); gameDataDir != "" {
			logger.Infof("Loading local file by gameconf_dir: %v ", gameDataDir)
			if err := gamedata.InitLocal(gameDataDir); err != nil {
				return err
			}
		}
		return nil
	}))

	// TransMgr：零配置即默认形态（DefaultShardCount 多分片 + 每键排队背压默认值）。
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(_ context.Context) error {
			sensitive_words.Init(conf.Get("base_cfg.dependencies.sensitive_words_file").String())
			if err := rds.RedisMgr.OnStart(nil); err != nil {
				return err
			}
			idGen, err := idgen.NewIDGen()
			if err != nil {
				return err
			}
			globals.IDGen = idGen
			var nacosConf net_conf.NacosConf
			_ = conf.Unmarshal("base_cfg.dependencies.nacos_conf", &nacosConf)
			if nacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", nacosConf.GroupName)
				if err = gamedata.InitNacos(nacosConf); err != nil {
					return err
				}
			}
			return err
		},
		// Stop 时关闭 Redis 连接池，返回聚合 Close error，消除连接泄漏。
		// 同时取消 Nacos gamedata 监听，回收监听 goroutine。
		OnStop: func(_ context.Context) error {
			gamedata.StopNet()
			return rds.RedisMgr.Close()
		},
	}

	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := mainsvrv1.NewMainC2SServiceSServer(&service.MainC2SServiceImpl{}, ssrpc.DefaultMWOptions{})
			return mainsvrv1.RegisterMainC2SServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	// SelfLogoutSender 注入：心跳过期淘汰改经事务串行执行，落盘与删除在 Logout
	// handler 内按 uid 串行键执行，消除 Tick 与业务 handler 对同一 *Role 的并发读写。
	// 必须在 router 起来之后（依赖 router.SelfBusId）。
	selfLogout := &bussvc.FuncComponent{
		ComponentName: "self_logout_sender",
		OnStart: func(_ context.Context) error {
			role.SelfLogoutSender = func(uid uint64, zone uint32, req *g1_protocol.LogoutReq) {
				globals.TransMgr.SendPbMsgToMyself(router.SelfBusId(), uid, uid, zone, g1_protocol.CMD_MAIN_LOGOUT_REQ, req)
			}
			return nil
		},
	}

	// 角色 Tick：原 OnTick 每分钟翻转触发一次 RoleMgr.Tick()。现替换为精确 1 分钟周期
	// 的 Task（NonOverlap 默认禁止重入），空闲服务不再每 10ms 被唤醒。
	roleTick := scheduler.New("role_tick", time.Minute, func(_ context.Context) error {
		globals.RoleMgr.Tick()
		return nil
	})

	// 角色落盘：原 OnShutdownExtra。作为 Drainer 全量落盘在线角色。
	//
	// 注册顺序说明（重要，Drain 顺序修复）：
	// runtime 对 Quiesce/Drain/Stop 一律按注册序的【逆序】执行（run.go drainComponents）。
	// 因此要让 roleFlush 在 TransMgr 排空【之后】执行（此时已无 handler 并发修改角色），
	// 必须把它注册在 transMgr【之前】。旧实现注册在最后，导致 FlushAllToDB 与
	// 在途事务并发读写同一 *Role，存在停机丢数据/数据竞争风险。
	roleFlush := &roleFlushComponent{}

	// Start 顺序：datetime（WithConfLoader 自动注册，隐含在最前）→ logger → admin
	// → tracing → 业务依赖 → SSRPC 注册 → roleFlush(Start 空操作) → TransMgr
	// → router/bus → SelfLogoutSender → roleTick(Task)。
	// Drain 逆序：roleTick → selfLogout → routerComp → transMgr → roleFlush
	// —— TransMgr 排空后才全量落盘角色。datetime_tick 必须最前：logger/xorm 等
	// 启动期即读 datetime，由 WithConfLoader 自动注册保证。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(
		businessDeps, registerHandlers, roleFlush, transMgr, routerComp,
		selfLogout, roleTick,
	)
	return app
}

// roleFlushComponent 在 Drain 阶段全量落盘在线角色（原 OnShutdownExtra）。
type roleFlushComponent struct{}

func (roleFlushComponent) Name() string                  { return "role_flush" }
func (roleFlushComponent) Start(_ context.Context) error { return nil }

// Drain 实现 runtime.Drainer：TransMgr 已排空，此时没有 handler 并发修改角色。
func (roleFlushComponent) Drain(_ context.Context) error {
	if _, failed := globals.RoleMgr.FlushAllToDB(); failed > 0 {
		return errors.New("failed to flush all roles to db on shutdown")
	}
	logger.Infof("================== mainsvr Stop =========================")
	return nil
}
func (roleFlushComponent) Stop(_ context.Context) error { return nil }
