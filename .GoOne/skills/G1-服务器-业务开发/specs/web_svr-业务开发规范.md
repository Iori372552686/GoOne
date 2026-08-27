# web_svr 业务开发规范

> 适用服务：`web_svr`（HTTP/Gin + 可选 gRPC 对外接口）
> 参考实现：`src/web_svr/app.go`、`src/web_svr/controller/`、`src/web_svr/globals/`
> 相关文档：[../SKILL.md](../SKILL.md)

---

## 一、服务定位

web_svr 是 GoOne 的**对外接口服务**（**非 bus 服务**，是唯一例外）：

- 提供 `WebApiService`（当前 Ping / MsgSecCheck）
- 启动 Gin HTTP 路由（从 `src/web_svr/controller` 挂载）+ 可选 gRPC listener
- 共享一个已 Seal 的 `Dispatcher`
- gRPC listener 附带 health + 可选 reflection
- 不参与 bus 事务循环

**不做**：bus 事务、玩家主数据（经 ssrpc 读其他服务）。

---

## 二、核心组件

`src/web_svr/app.go` 的 `NewApp()` 装配（与 bus 服务不同）：

| 组件 | 职责 |
|------|------|
| `webRuntimeComponent` | 实现 Component + Drainer + RuntimeErrorSource；启动 HTTP(Gin) + 可选 gRPC |

- `bussvc.MustNew("websvr", nil, ...)`（**readyCheck 传 nil**，非 bus 服务）
- gRPC listener 附带 health + 可选 reflection

---

## 三、开发规范

### 3.1 新增 HTTP 接口

1. 在 `common/game_proto/service/websvr.proto` 定义 rpc method + ssrpc option：

```proto
rpc NewApi(NewReq) returns (NewRsp) {
  option (goone.options.v1.ssrpc) = {
    http_path: "/v1/web/new-api"
    http_method: "POST"
    sign: true              // 是否需要签名
    comment: "新接口"
  };
}
```

2. 运行 `go run ./tools/cmd/genproto` + `./main.sh check-genproto`
3. 在 `src/web_svr/controller/` 实现 handler（经 `WrapHTTPGin` 挂到 `gin.IRoutes`）

> HTTP-only method 可不配 cmd。

### 3.2 新增 gRPC 接口

配 `grpc: true` option，经 `WrapGRPCUnary` 挂到 `grpc.Server`。当前无业务 method 显式 `grpc: true`。

### 3.3 配置要求

- `websvr.runtime.http_server.port` 必填 > 0（`web_gin.Config`：port/session_name/mode/auth_enable）
- `websvr.runtime.grpc_server.enabled` 时 `grpc_server.port` 必填 > 0（`GRPCServerConfig`：enabled/ip/port/reflection）
- `base_cfg.dependencies.db_instances` 非空
- admin 端口 fallback 8112

### 3.4 跨服务调用

web_svr 不持有自有数据，经 ssrpc client 读其他服务：

```go
mainCli := mainsvrv1.NewMainC2SServiceClient()
mainCli.SomeMethod(trans, &req)
```

---

## 四、自测要点

- HTTP 路由可达（`/v1/web/*`）
- 签名校验（`sign: true`）生效
- gRPC health/reflection（如启用）
- 优雅停机（`webRuntimeComponent` Drainer）
- 配置校验（http_server.port > 0；grpc 启用时 port > 0）
