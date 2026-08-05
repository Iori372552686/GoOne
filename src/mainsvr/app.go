package mainsvr

import (
	"context"
	"errors"
	"time"

	mainsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mainsvr/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/service"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// NewApp 用 runtime.App + Component 装配 mainsvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	// gamedata 本地目录加载作为 LoadConfig 的追加钩子，在校验通过后执行。
	app := runtime.MustNew("mainsvr", bussvc.WithConfLoader(func(_ context.Context) error {
		if gameDataDir := conf.Get("base_cfg.dependencies.game_data_dir").String(); gameDataDir != "" {
			logger.Infof("Loading local file by gameconf_dir: %v ", gameDataDir)
			if err := gamedata.InitLocal(gameDataDir); err != nil {
				return err
			}
		}
		return nil
	}))

	// 标准组件：服务名取 app.Name()，Start 时（LoadConfig 之后）自读 conf。
	logComp := bussvc.NewLoggerComponent(app)
	adminComp := bussvc.NewAdminComponent(app, router.ReadyCheck)
	tracing := bussvc.NewTracingComponent(app)

	// TransMgr：值语义配置，分片数由 Start 从 conf 读取（ShardCountConfKey），
	// <=0 回退、启动日志均在组件内部完成。
	transMgr := &bussvc.TransMgrComponent{
		Mgr:               globals.TransMgr,
		ShardCountConfKey: "mainsvr.capacity.trans_shard_count",
		Cfg: transaction.TransactionMgrConfig{
			MaxTrans:         misc.MaxTransNumber,
			MaxPendingPerKey: 100,
		},
	}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(_ context.Context) error {
			sensitive_words.Init(conf.Get("base_cfg.dependencies.sensitive_words_file").String())
			var dbs []redis.Config
			if err := conf.Unmarshal("base_cfg.dependencies.db_instances", &dbs); err != nil {
				return err
			}
			if err := rds.RedisMgr.InitAndRun(dbs); err != nil {
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
				// 经 lib/contrib/config/factory 构造配置中心 client，
				// 由 gamedata.InitRemote 统一加载+热更；构造/拉取失败返回 error。
				if err := gamedata.InitNacos(nacosConf); err != nil {
					return err
				}
			}
			return nil
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

	// 角色落盘：原 OnShutdownExtra。TransMgr 排空后没有 handler 并发修改角色，安全地
	// 全量落盘，避免 write-behind 防抖窗口内的变更在停机时丢失。作为 Drainer 在
	// TransMgr Drain 之后执行（注册顺序：roleFlush 在 transMgr 之后）。
	roleFlush := &roleFlushComponent{}

	// Start 顺序：datetime → logger → admin → tracing → 业务依赖 → SSRPC 注册
	// → TransMgr → router/bus → SelfLogoutSender → roleTick(Task) → roleFlush(Drainer)。
	// datetime_tick 放最前：logger/xorm 等启动期即读 datetime，需保证 ticker 已起。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(
		scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing,
		businessDeps, registerHandlers, transMgr, routerComp,
		selfLogout, roleTick, roleFlush,
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
