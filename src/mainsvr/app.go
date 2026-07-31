package mainsvr

import (
	"context"
	"errors"
	"time"

	mainsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mainsvr/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/service"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// NewApp 用 runtime.App + Component 装配 mainsvr。
func NewApp() *runtime.App {
	transMgr := &bussvc.TransMgrComponent{
		Mgr: globals.TransMgr,
		Cfg: func() transaction.TransactionMgrConfig {
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
	}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(_ context.Context) error {
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
		// V4 P0-05：Stop 时关闭 Redis 连接池，返回聚合 Close error，消除连接泄漏。
		OnStop: func(_ context.Context) error {
			return rds.RedisMgr.Close()
		},
	}

	// P1-03：用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := mainsvrv1.NewMainC2SServiceSServer(&service.MainC2SServiceImpl{}, ssrpc.DefaultMWOptions{})
			return mainsvrv1.RegisterMainC2SServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// P1-04：显式 DriverRegistry，只注册 rabbitmq。
	drivers := bus.NewDriverRegistry()
	drivers.MustRegister(rabbitmq.Driver())
	routerComp := &bussvc.RouterComponent{
		Common:   mainCommon,
		TransMgr: globals.TransMgr,
		Drivers:  drivers,
	}
	tracing := &bussvc.TracingComponent{
		ServiceName: "mainsvr",
		Cfg:         func() ssrpc.TracingConfig { return mainCommon().Tracing },
	}

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
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			c := mainCommon()
			return bussvc.LoggerConfig{Dir: c.LogDir, Level: c.LogLevel, Name: "mainsvr"}
		},
	}

	app := runtime.MustNew("mainsvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadMainConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			if gconf.MainSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.MainSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.MainSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			logger.Infof("gconf file load success for mainsvr")
			return nil
		}),
	)

	// P0-03：admin 在 LoadConfig 后用 WithAdminConfig 延迟读取端口/IP，复用
	// app.tracker。
	adminComp := runtime.NewAdminComponent(app,
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			c := mainCommon()
			return runtime.AdminConfig{
				Enabled: c.AdminEnabled,
				IP:      c.AdminIP,
				Port:    c.AdminPort,
				Pprof:   c.Pprof,
			}
		}),
		runtime.WithAdminServiceName("mainsvr"),
		runtime.WithAdminReadyCheck(router.ReadyCheck),
	)

	// Start 顺序（P0-03）：datetime → logger → admin → tracing → 业务依赖 → SSRPC 注册
	// → TransMgr → router/bus → SelfLogoutSender → roleTick(Task) → roleFlush(Drainer)。
	// datetime_tick 放最前：logger/xorm 等启动期即读 datetime，需保证 ticker 已起。
	// P1-07：用 MustRegister 一次注册全部组件。
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

// mainCommon 从 gconf 产出 mainsvr 的 bus 服务共享配置段。
func mainCommon() bussvc.Common {
	c := &gconf.MainSvrCfg
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
