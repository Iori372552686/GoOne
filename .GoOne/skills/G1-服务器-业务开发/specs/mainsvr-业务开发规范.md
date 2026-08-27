# mainsvr 业务开发规范

> 适用服务：`mainsvr`（核心业务逻辑）
> 参考实现：`src/mainsvr/app.go`、`src/mainsvr/role/`、`src/mainsvr/globals/`
> 相关文档：[../SKILL.md](../SKILL.md) | [03-玩家服务数据库协议规范文档.md](../../../spec/03-玩家服务数据库协议规范文档.md)

---

## 一、服务定位

mainsvr 是 GoOne 的**核心业务服务**，承载玩家面向的业务逻辑：

- 玩家状态经 `globals.RoleMgr` 管理（登录加载、在线操作、离线落盘）
- 处理 `MainC2SService` 的全部 C2S 业务（Login/Logout/HeartBeat/ChangeName/房间操作/DoBet/...）
- 玩家主数据落 Redis hash（`persist_hash.go`），关键字段落 MySQL `role_info`（经 mysqlsvr ssrpc）
- 按 UID 粘性路由（`SvrRouterRule_Hash_UID`）

**不做**：网络监听（connsvr 负责）、房间 tick（roomcentersvr 负责）、持久化 MySQL 直写（mysqlsvr 负责）。

---

## 二、核心组件

`src/mainsvr/app.go` 的 `NewApp()` 装配：

| 组件 | 职责 |
|------|------|
| `businessDeps`（FuncComponent） | 加载敏感词、Redis、`idgen.NewIDGen()`、gamedata（本地或 Nacos） |
| `selfLogout`（FuncComponent） | 注入 `role.SelfLogoutSender`（心跳淘汰经事务串行） |
| `registerHandlers`（ssrpc RegistryComponent） | 注册 `MainC2SServiceSS`（玩家业务） |
| `roleTick`（`scheduler.New("role_tick", time.Minute, ...)`） | 每分钟 `RoleMgr.Tick()` 落盘 |
| `transMgr`（TransMgrComponent） | 事务管理（Drainer） |
| `roleFlushComponent`（Drainer） | 停机时全量落盘角色 |
| `routerComp`（RouterComponent） | bus 路由 |

---

## 三、开发规范

### 3.1 新增 C2S 业务方法

1. 在 `common/game_proto/service/mainsvrc2s.proto` 定义 rpc method + ssrpc option（`cmd_name` 引用 `CMD_MAIN_*`）
2. 如需新 cmd，在 `common/game_proto/core/cmd.proto` 的 `CMD` 枚举加（`MainSvr=2` 号段 `0x20000+`）
3. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
4. 在 `src/mainsvr/service/` 实现 handler（`MainC2SServiceImpl`）

```go
func (s *MainC2SServiceImpl) NewMethod(ctx *ssrpc.Context, req *g1_protocol.NewReq) (*g1_protocol.NewRsp, error) {
    // 经 ctx 拿 UID/Role
    // 业务逻辑（操作 RoleMgr）
    // 返回 rsp, gerr.New(...) 或 rsp, nil
}
```

### 3.2 玩家数据操作

- 经 `globals.RoleMgr` 获取/操作 Role
- 落盘：`Role.SaveToDB(trans)`（Redis hash 增量）+ `Role.SaveToMysql(trans)`（role_info）
- 新业务模块：接入 `src/mainsvr/role/persist_hash.go` 的 hash field 编排
- 关键数据（货币/道具）先落库再更新内存

### 3.3 并发约束

- handler 保持 key-local（同 UID 串行，由 TransactionMgr sharded 保证）
- 禁止 handler 与后台 goroutine 共享可变对象（参考 `SelfLogoutSender` 模式）
- 禁止依赖全局单线程排序（不同 UID 并发）

### 3.4 跨服务调用

经 ssrpc client 调其他服务：

```go
// 调 mysqlsvr 落库
mysqlCli := mysqlsvrv1.NewMysqlServiceClient()
mysqlCli.UpdateRoleInfo(trans, &req)

// 调 roomcentersvr
rcCli := roomcenterv1.NewRoomCenterInnerServiceClient()
rcCli.QuickStart(trans, &req)
```

---

## 四、自测要点

- Role 加载/落盘/停机全量落盘流程完整
- Redis hash field 无冲突
- serial key 串行正确（同 UID 请求串行）
- 关键数据（货币/道具）先落库
- 心跳淘汰（`ClientExpiryThreshold = 6 × 心跳`）经事务串行
- 集成测试 `GOONE_INTEGRATION=1`
