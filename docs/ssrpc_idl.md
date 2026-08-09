## GoOne ssrpc / IDL 现状说明

本文描述当前主线已经落地的 ssrpc / IDL 方案，重点是“现在仓库里真正有哪些能力、如何接入、边界在哪”。

### 1. 目标

GoOne 的 ssrpc 方案把：

- proto service + 代码生成的 IDL 驱动模式
- middleware / transport 分层
- GoOne 现有的 `TransactionMgr + SSPacket` 执行模型

组合成一套统一的 RPC runtime。

当前主线最成熟的是：

- SSPacket unary（服务间主通路）
- HTTP Gin 挂载（`web_svr`）
- WS/CSPacket 挂载（`connsvr` 登录前置）

gRPC unary 与 server-streaming runtime / 代码生成已经具备（option 字段 `grpc` / `grpc_service` 在 `api/proto/goone/options/v1/options.proto` 中定义）。**当前主线尚无 method 显式声明 `grpc: true`**——`common/game_proto/service/websvr.proto` 的方法仍走 HTTP；`web_svr` 已预留可配置的 gRPC listener 挂载位，业务 method 按需 opt-in 即可。

### 2. 当前代码布局

- repo-owned proto 输入：`api/proto/goone/**`、`api/proto/game/**`，以及存在时的 `api/proto/web/**`
- protocol-owned service proto：`common/game_proto/**/*.proto` 中声明了 `service` 的文件
- 生成产物：`api/gen/**`
- runtime：`lib/service/ssrpc/**`
- 生成器：`tools/protoc-gen-goone/**`
- 典型接入点：
  - `src/*/app.go`
  - `src/connsvr/ws_login.go`
  - `src/web_svr/controller/router.go`

业务 service proto 目前主要以 `common/game_proto/service/*.proto` 为 source-of-truth；例如 `common/game_proto/service/mainsvrc2s.proto`、`common/game_proto/service/websvr.proto`。

### 3. method option（当前实现）

`goone.options.v1.ssrpc` 当前已支持以下字段：

- `cmd`：请求 cmd，直接写数值
- `cmd_enum`：请求 cmd，引用 `goone.cmd.v1.CMD`
- `cmd_name`：请求 cmd，引用 `g1_protocol` 的 Go 常量名
- `cmd_resp`：显式响应 cmd；默认仍按 `cmd+1`
- `one_way`：只发不回
- `uid_lock`：按 uid 串行，runtime 默认使用 striped locker
- `auth`：要求鉴权
- `sign`：要求验签
- `trace_tags`：额外 trace tag，`k=v` 形式
- `timeout_ms`：method 级超时
- `http_path` / `http_method`：HTTP 绑定
- `ws`：生成 WS/CSPacket 绑定
- `grpc`：生成 gRPC unary / server-streaming 绑定
- `grpc_service`：覆盖注册到 `grpc.Server` 的 service 名
- `comment`：用于生成注释 / method 展示名

cmd 绑定优先级：

- `cmd != 0` -> 用 `cmd`
- 否则 `cmd_enum != 0` -> 用 `cmd_enum` 数值
- 否则 `cmd_name != ""` -> 用 `g1_protocol.<cmd_name>`

当前语义边界：

- HTTP-only method 可以只写 `http_path`，不配置 cmd
- `ws` 目前仍要求有 cmd 绑定
- gRPC unary method 可以不配置 cmd
- gRPC server-streaming method 当前必须是 grpc-only，不应同时配置 `cmd` / `http_path` / `ws`
- 当前只支持 unary 与 server-streaming；client-streaming / bidi-streaming 尚未支持
- `grpc_service` 未配置时，默认使用完整 proto service 名，例如 `game.mainsvr.v1.MainC2SService`

### 4. 生成命令

推荐入口：

```bash
go run ./tools/cmd/genproto
```

`tools/cmd/genproto` 当前会：

- 必要时先确保 `cmd.proto` 存在
- 构建 `protoc-gen-go` 与 `protoc-gen-goone`
- 扫描 repo-owned proto 输入目录
- 扫描 `common/game_proto` 下真正声明了 `service` 的 proto
- 分两次调用 `protoc`，避免 repo proto 与 protocol proto 的消息符号冲突

一键包装：

- `./scripts/proto_goone.sh`
- `./scripts/proto_goone.ps1`

它们会先生成 `common/protocol/**`，再生成主仓 `api/gen/**`。

校验生成物：

- 默认：`./main.sh check-genproto` 或 `.\scripts\check_genproto.ps1`
- 全量：`./main.sh check-genproto --full` 或 `.\scripts\check_genproto.ps1 -Full`

如果你改的是 `common/game_proto` 里的共享消息，而不只是 service proto，合并前应执行 full 检查。

### 5. 生成代码长什么样

对于每个带 `ssrpc` option 的 service，生成器当前会产出：

- `<Service>SS`：类型安全的服务接口
- `Unimplemented<Service>SS`
- `Default<Service>SSMiddlewares(...)`
- `New<Service>SServer(...)`
- `Register<Service>ToTransactionMgr(...)`
- `Register<Service>ToGin(...)`：当 method 配置了 `http_path`
- `Register<Service>ToWS(...)`：当 method 配置了 `ws: true`
- `Register<Service>ToGRPC(...)`：当 method 配置了 `grpc: true`，支持 unary 与 server-streaming
- `Register<Service>ToDispatcher(...)`
- `<Service>Client`

client stub 当前规则：

- 普通 request/response：生成 `Method(ctx, req)` 与 `MethodByRouter(ctx, routerId, req)`
- one-way method：额外生成 `ByBusId` / `Simple` / `ByRouterSimple` 等 helper
- grpc-only method 若没有 cmd 绑定，不会生成 cmd-based client stub

### 6. runtime 行为

当前 runtime 统一由 `lib/service/ssrpc/**` 承担：

- SSPacket：`WrapUnary`
- HTTP：`WrapHTTPGin`
- WS/CSPacket：`WrapWS`
- gRPC unary：`WrapGRPCUnary`
- gRPC server-stream：`WrapGRPCServerStream` / `WrapGRPCServerStreamTyped`
- 统一注册中心：`ssrpc.Dispatcher`

当前实现要点：

- wrapper 在初始化阶段预构建 middleware chain，不再在每次请求时重复拼装
- method 级元信息统一挂在 `ssrpc.MethodDesc`
- `ctx.ParseMsg` 失败统一返回 `ERR_MARSHAL`
- 非 `ssrpc.Error` 默认映射为 `ERR_INTERNAL`
- `cmd_resp` 会优先走 `SendMsgBackWithCmd`
- `Transaction.waitRsp` 以 `CmdSeq` 为主做响应关联；若响应 cmd 不是 `cmd+1`，会告警但仍尝试解码

默认中间件链通过 `New<Service>SServer(..., ssrpc.DefaultMWOptions{...})` 注入，当前包含：

- `Recover()`
- `Logging()`
- `TraceWith(...)`
- `AuthWith(...)`
- `SignWith(...)`
- `UIDLockAttach(...)`
- `Metrics(...)`
- `MCPAttach(...)` / `MCPGuardWith(...)`（仅当配置了 MCP）

### 7. 推荐接入方式

#### 7.1 SSPacket / TransactionMgr

```go
srv := mainsvrv1.NewMainC2SServiceSServer(
  &service.MainC2SServiceImpl{},
  ssrpc.DefaultMWOptions{},
)

d := ssrpc.NewDispatcher()
mainsvrv1.RegisterMainC2SServiceToDispatcher(d, srv)
d.RegisterToTransactionMgr(globals.TransMgr)
```

当前 `mainsvr`、`infosvr`、`mysqlsvr`、`roomcentersvr`、`connsvr` 都已经切到这一类写法。

#### 7.2 HTTP / Gin

```go
srv := websvrv1.NewWebApiServiceSServer(&service.WebApiServiceImpl{}, ssrpc.DefaultMWOptions{
  Sign: service.NewHTTPSignVerifier(...),
})

d := ssrpc.NewDispatcher()
websvrv1.RegisterWebApiServiceToDispatcher(d, srv)
d.MountGin(router)
```

#### 7.3 WS / CSPacket

`connsvr` 的登录前置路径当前通过 `Register<Service>ToWS(...)` 挂载到 `globals.ClientPacketDispatcher`。

#### 7.4 gRPC

当 proto method 配置 `grpc: true` 时，生成器会按 proto method 语义产出：

- 普通 unary method：`RegisterGRPCUnary(...)`
- `server_streaming = true` method：`RegisterGRPCStream(...)`

当前仓库里**尚无 method 声明 `grpc: true`**（`websvr.proto` 的 `Ping` / `MsgSecCheck` 均走 HTTP）。若要在某个 method 上启用 gRPC，在 proto option 中加 `grpc: true` 即可，例如：

```proto
rpc MyMethod(MyReq) returns (MyRsp) {
  option (goone.options.v1.ssrpc) = {
    grpc: true
  };
}
```

unary 接入示例：

```go
srv := mysvrv1.NewMyServiceSServer(&service.MyServiceImpl{}, ssrpc.DefaultMWOptions{})

d := ssrpc.NewDispatcher()
mysvrv1.RegisterMyServiceToGRPC(d, srv)
d.MountGRPC(grpcSrv)
```

`web_svr` 当前已经把这层 wiring 接到 app 中，开启方式类似：

```yaml
websvr:
  grpc_server:
    enabled: true
    ip: ""
    port: 10002
```

当前 `web_svr` 的 gRPC listener 还额外挂了：

- 标准 `grpc.health.v1.Health`
- gRPC reflection（便于本地 `grpcurl` / 调试工具发现服务）

server-streaming method 的生成接口签名为：

```go
Watch(ctx *ssrpc.Context, req *mypb.WatchReq, stream *ssrpc.ServerStream[*mypb.WatchRsp]) error
```

如果你需要覆盖默认 proto service 名，可在 option 中配置：

```proto
option (goone.options.v1.ssrpc) = {
  cmd_name: "CMD_MAIN_LOGIN_REQ"
  grpc: true
  grpc_service: "custom.grpc.v1.AuthService"
};
```

### 8. 当前边界与未覆盖项

以下内容在当前实现里要明确区分：

- 当前主线**尚无 method 声明 `grpc: true`**；`web_svr` 的 gRPC listener（`websvr.grpc_server`）已就绪但当前注册 0 个 gRPC service，业务 method 按需 opt-in
- 生成 gRPC 绑定不等于所有服务都会自动对外监听；当前 `web_svr` 已通过 `websvr.grpc_server` 配置接好示例，其它服务仍需各自挂载
- generator 现在支持 gRPC unary 与 server-streaming 自动注册
- client-streaming / bidi-streaming 仍未支持
- `ws` method 仍要求有 cmd 绑定
- 当前 client stub 仍基于 `cmd_handler.IContext`，而不是主流 RPC 常见的 `context.Context + CallOption`

### 9. 实践建议

- 优先使用 `cmd_enum` 或 `cmd_name`，避免手写裸数值
- 优先用 `New<Service>SServer(...)`，不要在 app 层手拼 middleware slice
- 优先通过 `Register<Service>ToDispatcher(...)` 作为统一入口，再分别 `RegisterToTransactionMgr` / `MountGin` / `MountGRPC`
- proto 改动后至少执行一次 `check-genproto`
- 若改动包含共享消息，执行 full 检查


