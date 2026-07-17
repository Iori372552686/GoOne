package mainsvr

import (
	"context"
	"errors"
	"fmt"
	"time"

	mainsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mainsvr/v1"
	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
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
	}

	registerHandlers := &bussvc.FuncComponent{
		ComponentName: "ssrpc_register",
		OnStart: func(_ context.Context) error {
			srv := mainsvrv1.NewMainC2SServiceSServer(&service.MainC2SServiceImpl{}, ssrpc.DefaultMWOptions{})
			d := ssrpc.NewDispatcher()
			mainsvrv1.RegisterMainC2SServiceToDispatcher(d, srv)
			d.RegisterToTransactionMgr(globals.TransMgr)
			return nil
		},
	}

	routerComp := &bussvc.RouterComponent{
		Common:   mainCommon,
		TransMgr: globals.TransMgr,
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

	app, err := runtime.New("mainsvr",
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
	if err != nil {
		panic(fmt.Sprintf("runtime.New mainsvr: %v", err))
	}

	// Start 顺序：tracing → 业务依赖 → TransMgr → SSRPC 注册 → router/bus →
	// SelfLogoutSender → roleTick(Task) → roleFlush(Drainer)。
	// 逆序用于 Quiesce/Drain/Stop：roleFlush 先排空落盘，再 TransMgr 排空，再 router。
	for _, c := range []runtime.Component{tracing, businessDeps, transMgr, registerHandlers, routerComp, selfLogout, roleTick, roleFlush} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("mainsvr register %s: %v", c.Name(), err))
		}
	}
	_ = misc.ServerType_MainSvr
	return app
}

// roleFlushComponent 在 Drain 阶段全量落盘在线角色（原 OnShutdownExtra）。
type roleFlushComponent struct{}

func (roleFlushComponent) Name() string { return "role_flush" }
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
