# mysqlsvr 业务开发规范

> 适用服务：`mysqlsvr`（MySQL 持久化服务）
> 参考实现：`src/mysqlsvr/app.go`、`src/mysqlsvr/manager/`、`src/mysqlsvr/service/`
> 相关文档：[../SKILL.md](../SKILL.md) | [04-游戏微服务数据库协议规范文档.md](../../../spec/04-游戏微服务数据库协议规范.md)

---

## 一、服务定位

mysqlsvr 是 GoOne 的**关系型持久化服务**：

- 提供 `MysqlService`（UpdateRoleInfo / SearchRole / Update / QueryRoomInfo / QueryPlayerInfo / QueryGameInfo）
- 经 `lib/db/xorm`（主从 `EngineGroup`）访问 MySQL
- 按 RouterID 路由（`SvrRouterRule_Hash_RouterID`）
- 异步写库（`async.NewAsyncPool(15)`，按 `id % 15` 分发）

**不做**：玩家主数据管理（mainsvr 的 Redis hash）、业务逻辑（mainsvr/roomcentersvr）。

---

## 二、核心组件

`src/mysqlsvr/app.go` 的 `NewApp()` 装配：

| 组件 | 职责 |
|------|------|
| `ormDeps`（FuncComponent） | 启动 async worker + `OrmMgr.InitAndRun` |
| `registerHandlers`（ssrpc RegistryComponent） | 注册 `MysqlServiceSS` |
| `transMgr`（TransMgrComponent） | 事务管理（Drainer） |
| `routerComp`（RouterComponent） | bus 路由 |

---

## 三、开发规范

### 3.1 新增持久化接口

1. 在 `common/game_proto/service/mysqlservice.proto` 定义 rpc method + ssrpc option（inner cmd，MsgType=1，号段 `0x41000+`）
2. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
3. 在 `src/mysqlsvr/service/` 实现 handler

### 3.2 新增 ORM 表

1. 在 `common/game_proto/core/` 定义表 struct proto（message 字段对应列）
2. 运行 proto 生成
3. 把 struct 注册到 `src/mysqlsvr/manager/table_mgr.go` 的 `tables` 列表：

```go
var tables = []interface{}{
    new(g1_protocol.MysqlTexasRoomInfo),
    new(g1_protocol.MysqlTexasPlayerInfo),
    new(g1_protocol.MysqlTexasGameInfo),
    // new(g1_protocol.YourNewTable),  // 新增
}
```

`OrmMgr` 启动时 `SyncTables` 自动建表。

### 3.3 裸 SQL（如 role_info）

```sql
INSERT INTO role_info VALUES (?, ?)        -- uid, name
UPDATE role_info SET name = ? WHERE uid = ?
SELECT uid FROM role_info
```

### 3.4 异步写库

高频写入经 `async.NewAsyncPool(15)` 异步落库，避免阻塞事务。

### 3.5 配置要求

- `mysqlsvr.identity.self_bus_id` 必填
- `base_cfg.dependencies.orm_instances` 非空（校验在 `module/conf/registry.go`）

---

## 四、自测要点

- ORM 表建表成功（`SyncTables`）
- 主从读写分离正确
- 异步写库不阻塞事务（`ASYNC_COUNT=15`）
- 集成测试需真实 MySQL（`GOONE_INTEGRATION=1`）
