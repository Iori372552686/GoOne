# roomcentersvr 业务开发规范

> 适用服务：`roomcentersvr`（房间生命周期/tick）
> 参考实现：`src/roomcentersvr/app.go`、`src/roomcentersvr/globals/`、`src/roomcentersvr/logic/`、`src/roomcentersvr/room_ai/`
> 相关文档：[../SKILL.md](../SKILL.md) | [04-游戏微服务数据库协议规范文档.md](../../../spec/04-游戏微服务数据库协议规范.md)

---

## 一、服务定位

roomcentersvr 是 GoOne 的**房间调度服务**（当前承载德州扑克玩法）：

- 提供 `RoomCenterInnerService`（Tick / RoomList / QuickStart / UpdateRoomInfo / DelRoomInfo）
- 管理 `RoomListMgr`（房间列表、创建/销毁）
- 房间 tick（`roomTick` 5s）+ 房间持久化（`roomPersist` 10s）
- 按 RouterID 路由（`SvrRouterRule_Hash_RouterID`）
- **同键 backpressure 覆盖为 200**（默认 100，房间高频同键包）
- 从 Redis 恢复房间快照 + `room_ai.OnAiInitRoom()`

**不做**：玩家主数据（mainsvr）、网络监听（connsvr）。

> 注：德州玩法的游戏逻辑在 `TexasGameSvr`（`ServerType_TexasGameSvr = 0x50`，号段 `0x501000+`）。

---

## 二、核心组件

`src/roomcentersvr/app.go` 的 `NewApp()` 装配：

| 组件 | 职责 |
|------|------|
| `transMgr`（TransMgrComponent） | **显式 `MaxPendingPerKey: 200`**（覆盖默认 100） |
| `cmdBacklist` | `logger.RegisterCmdBacklist(CMD_ROOM_CENTER_INNER_TICK_REQ)`（tick 日志降噪） |
| `roomInit`（FuncComponent） | `RoomListMgr.Init` + 从 Redis 恢复快照 + `room_ai.OnAiInitRoom()` |
| `registerHandlers`（ssrpc RegistryComponent） | 注册 `RoomCenterInnerServiceSS` |
| `roomTick`（`scheduler.New`，5s） | 房间 tick（精确周期 Task） |
| `roomPersist`（`scheduler.New`，10s） | 房间持久化到 Redis |
| `roomFlushComponent`（Drainer） | 停机全量落盘房间 |
| `routerComp`（RouterComponent） | bus 路由 |

---

## 三、开发规范

### 3.1 新增房间接口

1. 在 `common/game_proto/service/roomcentersvr.proto` 定义 rpc method + ssrpc option（inner cmd，号段 `0xB1000+`）
2. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
3. 在 `src/roomcentersvr/service/` 实现 handler

### 3.2 房间逻辑

- 房间状态经 `RoomListMgr` 管理
- 房间内逻辑放 `src/roomcentersvr/logic/`（`.golangci.yml` 排除该目录的存量债务）
- 房间操作（CreateRoom/QuickStart/DoBet 等）经 ssrpc 或从 mainsvr 调入

### 3.3 tick 与持久化

- `roomTick`（5s）：房间状态推进（德州牌局阶段）
- `roomPersist`（10s）：房间快照写 Redis
- `roomFlushComponent`（Drainer）：停机全量落盘
- 房间明细落 MySQL（Texas 三表）经 ssrpc 调 mysqlsvr

### 3.4 并发约束

- handler 保持 key-local（同 RouterID 串行，由 TransactionMgr sharded 保证）
- 同键队列 backpressure = 200（房间高频）
- tick handler 加入日志黑名单（`CMD_ROOM_CENTER_INNER_TICK_REQ`）

### 3.5 配置要求

- `roomcentersvr.identity.self_bus_id` 必填
- admin 端口 fallback 8111

---

## 四、自测要点

- 房间创建/销毁/列表流程完整
- tick（5s）与 persist（10s）周期正确
- 停机 `roomFlushComponent` 全量落盘
- 从 Redis 恢复快照 + AI 初始化房间
- 同键 backpressure 200 生效
- 房间明细经 mysqlsvr 落 Texas 三表
