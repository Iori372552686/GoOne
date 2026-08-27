# infosvr 业务开发规范

> 适用服务：`infosvr`（轻量缓存/查询服务）
> 参考实现：`src/infosvr/app.go`、`src/infosvr/globals/`
> 相关文档：[../SKILL.md](../SKILL.md)

---

## 一、服务定位

infosvr 是 GoOne 的**缓存与查询服务**：

- 提供 `InfoService`（GetBriefInfo / GetIconDesc / SetBriefInfo）
- 玩家 BriefInfo / IconDesc 缓存于 Redis
- 按 UID 粘性路由（`SvrRouterRule_Hash_UID`）
- 结构最简，是**新服务的脚手架模板**（`tools/cmd/scaffold` 生成结构同 infosvr）

**不做**：玩家主数据管理（mainsvr）、持久化 MySQL（mysqlsvr）。

---

## 二、核心组件

`src/infosvr/app.go` 的 `NewApp()`（最简范例）：

| 组件 | 职责 |
|------|------|
| `redisDeps`（FuncComponent） | Redis 依赖初始化 |
| `registerHandlers`（ssrpc RegistryComponent） | 注册 `InfoServiceSS` |
| `transMgr`（TransMgrComponent） | 事务管理（Drainer） |
| `routerComp`（RouterComponent） | bus 路由 |

---

## 三、开发规范

### 3.1 新增查询接口

1. 在 `common/game_proto/service/infoservice.proto` 定义 rpc method + ssrpc option
2. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
3. 在 `src/infosvr/service/` 实现 handler（`InfoServiceImpl`）

### 3.2 缓存操作

- 经 `lib/db/redis`（`rds.RedisMgr.GetClient(instID)`）读写缓存
- 必须设置 TTL（除非明确永久）
- 未命中时回源（经 ssrpc 调 mainsvr 或 mysqlsvr）

### 3.3 配置要求

- `infosvr.identity.self_bus_id` 必填
- `base_cfg.runtime.register_addr` / `bus_mq_addr` 必填
- `base_cfg.dependencies.db_instances` 非空（校验在 `module/conf/registry.go`）

---

## 四、自测要点

- 缓存命中/未命中路径
- Redis 读写正确
- TTL 生效
- 按 UID 路由到固定实例

### 新服务脚手架

```bash
go run tools/cmd/scaffold -name mysvr    # 生成 src/mysvrsvr/（结构同 infosvr）
```
