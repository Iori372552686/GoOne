# connsvr 业务开发规范

> 适用服务：`connsvr`（TCP/WS/KCP 网关）
> 参考实现：`src/connsvr/app.go`、`src/connsvr/globals/`
> 相关文档：[../SKILL.md](../SKILL.md) | [02-通讯协议规范文档.md](../../../spec/02-通讯协议规范文档.md)

---

## 一、服务定位

connsvr 是 GoOne 的**网络网关**，负责客户端接入与消息转发：

- 监听 TCP（`listen_port+1`）/ WS（`listen_port`）/ KCP（`kcp_port`），三种传输共享 `SessionHub`
- 解析 `CSPacketHeader`（28 字节大端包头）+ protobuf body
- 按 cmd 的 `ServerType` 经 `lib/service/router` 转发到目标 bus 服务
- 提供 `AdmissionController` 过载保护（off/shadow/enforce）
- 提供 S2S 接口 `ConnService`（KickOut/Broadcast）供其他服务调用

**不做**：业务逻辑、玩家数据持久化（交给 mainsvr 等）。

---

## 二、核心组件

`src/connsvr/app.go` 的 `NewApp()` 装配：

| 组件 | 职责 |
|------|------|
| `signRestDeps`（FuncComponent） | 加载 `http_sign` + `rest_api_config` 配置 |
| `registerHandlers`（ssrpc RegistryComponent） | 注册 `ConnServiceSS`（KickOut/Broadcast） |
| `transMgr`（TransMgrComponent） | 事务管理（Drainer） |
| `routerComp`（RouterComponent） | bus 路由，传 `onRecvSSPacket`（下行包短路回客户端） |
| `gatewayComponent` | 启动 TCP/WS/KCP 监听，管理 SessionHub + AdmissionController（实现 Component + Quiescer + Drainer） |

---

## 三、开发规范

### 3.1 新增 S2S 接口（如 KickOut/Broadcast 扩展）

1. 在 `common/game_proto/service/connservice.proto` 定义 rpc method + ssrpc option
2. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
3. 在 `src/connsvr/service/` 实现 handler（`ConnServiceImpl`）
4. 确认 `app.go` 的 `registerHandlers` 已注册

### 3.2 网关层改动（监听/会话/过载）

- 修改 `gatewayComponent`，注意实现 `Quiesce`（停接新连接）+ `Drain`（等待 `SessionTracker.WaitSessions` 排空）
- 配置项在 `connsvr.runtime`（`listen_port`、`tcp_impl_type` gonon/gnet、`kcp_port`）与 `connsvr.capacity`（`max_connections`、`max_unauthenticated_connections`、`connection_rate`、`login_rate`、`max_inflight`、`max_inflight_per_method`、`overload_mode`）
- 改配置 struct 加到 `module/gconf/config.go` 的 `ConnSvr`，校验加到 `module/conf/registry.go`

### 3.3 登录接入

connsvr 处理 WS 登录前置（`ws: true` 的 ssrpc method，如 mainsvr 的 `Login`），鉴权后绑定 UID 到会话。

---

## 四、自测要点

- 三种传输（TCP/WS/KCP）连通性
- `CSPacketHeader` 解析正确（Version/PassCode/Seq/Uid/Cmd/BodyLen）
- KickOut/Broadcast 经 ssrpc 调用生效
- `AdmissionController` 三种模式（off/shadow/enforce）行为正确
- 优雅停机：SessionTracker 排空在途会话
