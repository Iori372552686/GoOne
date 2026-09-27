package mysqlsvr

import (
	"context"
	"fmt"

	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	gormdb "github.com/Iori372552686/GoOne/lib/db/gorm"
	"github.com/Iori372552686/GoOne/lib/db/shard"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/repository"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/service"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// NewApp 用 runtime.App + Component 装配 mysqlsvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	app := bussvc.MustNew("mysqlsvr", router.ReadyCheck, bussvc.WithConfLoader())

	// 标准组件（datetime/logger/admin/tracing）由 bussvc.MustNew 集中注册。

	// TransMgr：Start 启动分片 worker；Drain 排空在途事务（受 ctx 超时约束）。
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}
	repo := repository.NewWithShard(globals.DBMgr, roleShardRule())

	// SSRPC 注册：必须在 Seal（于 router/bus Start 之前）完成，且在 TransMgr Start
	// 之后（RegisterToTransactionMgr 依赖已 InitAndRun 的 TransMgr）。这里作为
	// FuncComponent，注册顺序置于 TransMgr 之后、Router 之前。
	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := mysqlsvrv1.NewMysqlServiceSServer(service.NewMysqlServiceImpl(repo), ssrpc.DefaultMWOptions{})
			return mysqlsvrv1.RegisterMysqlServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// ORM 依赖：Start 初始化 ORM Engine + role_data 分表建表/迁移；Stop 关闭。
	roleRule := roleShardRule()
	ormDeps := &bussvc.FuncComponent{
		ComponentName: "orm_deps",
		OnStart: func(ctx context.Context) error {
			var ormConf []gormdb.Config
			if err := conf.Unmarshal("base_cfg.dependencies.orm_instances", &ormConf); err != nil {
				return err
			}
			if err := globals.DBMgr.InitAndRun(ctx, ormConf); err != nil {
				return err
			}
			db, err := globals.DBMgr.GetDB()
			if err != nil {
				return err
			}
			// role_data 物理表按分片规则显式建名（TableShards<=1 时即基表）。
			if err := shard.EnsureTables(ctx, db, roleRule, &g1_protocol.MysqlRoleData{}); err != nil {
				return err
			}
			// 存量未分片数据自动迁移（TableShards>1 且基表存在时生效；逐行 INSERT、
			// 成功后基表改名为 role_data__legacy 保留，幂等可重跑）。
			var legacy []g1_protocol.MysqlRoleData
			keyOf := func(i int) uint64 { return legacy[i].Uid }
			migrated, err := shard.MigrateUnsharded(ctx, db, roleRule, &legacy, keyOf)
			if err != nil {
				return fmt.Errorf("migrate role_data to sharded tables: %w", err)
			}
			if migrated > 0 {
				logger.Infof("role_data migrated to sharded tables {rows:%d, legacy:%s}",
					migrated, shard.LegacyName(roleRule.TableBase))
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			// 同时关闭 ORM Engine（此前只关 async worker，Engine 靠 OS 回收）。
			err := globals.DBMgr.Close()
			logger.Infof("================== mysqlsvr Stop =========================")
			return err
		},
	}

	// DriverRegistry 只注册 rabbitmq。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	// 注册顺序即 Start 顺序：datetime 周期刷新（WithConfLoader 自动注册，隐含在最前）
	// → logger → admin → tracing → orm 依赖（DB pool）→ SSRPC 注册（必须在
	// TransMgr.InitAndRun 之前，因 RegisterToTransactionMgr 调 RegisterCmd）→
	// TransMgr → router/bus。
	// Drain 逆序：router → TransMgr（排空 handler）→ ormDeps（关 DB pool）。
	// 旧德州项目的异步写库 worker 池（manager 包）已随 Update RPC 一并移除。
	app.MustRegister(
		ormDeps, registerHandlers, transMgr, routerComp,
	)
	return app
}

// roleShardRule 从配置装配 role_data 分表规则。缺省不分表（TableShards<=1）。
func roleShardRule() *shard.Rule {
	rule := &shard.Rule{
		Name:        "role_data",
		TableBase:   "role_data",
		TableShards: conf.Get("mysqlsvr.capacity.role_table_shards").Int(),
	}
	if err := rule.Validate(); err != nil {
		logger.Errorf("invalid role_data shard rule, fallback to unsharded | %v", err)
		return &shard.Rule{Name: "role_data", TableBase: "role_data"}
	}
	return rule
}
