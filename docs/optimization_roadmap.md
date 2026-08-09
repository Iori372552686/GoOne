# GoOne 核心现代化 P0/P1/P2 执行计划

> 状态：P0 + P1 已完成并合入 dev（2026-07-20），真实环境联调通过；P2 待进入条件  
> 创建日期：2026-07-16  
> 架构依据：[architecture_review_2026-07-v2.md](architecture_review_2026-07-v2.md)  
> 执行原则：测试先行、按依赖顺序、每批可验证、最终直接替换旧 API

## 1. 目标

将当前分散在 application、bootstrap、busapp 和各服务 app.go 中的初始化与关闭 Hook，统一为：

- 单一 App.Run。
- 显式 Module 注册。
- Component Start/Stop 生命周期。
- Ready、Allocated、Draining 状态机。
- Gateway、Bus、Transaction、HTTP/gRPC 的两阶段排空。
- 不可变配置快照与安全局部重载。
- 应用显式选择所需 Driver。
- 可复现的性能基线和回归门禁。

目标主入口：

~~~go
func main() {
    flag.Parse()

    if err := mainsvr.NewApp().Run(context.Background()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
~~~

## 2. 优先级

| 优先级 | 定义 | 发布要求 |
|---|---|---|
| P0 | 生命周期正确性、启动回滚、注册安全、配置安全、排空和直接迁移 | 全部完成后才能发布新生命周期 |
| P1 | 有基线支撑的性能、安全、可观测性、CI 和风格收口 | 可在 P0 后分批发布 |
| P2 | gnet v2、Agones、Driver 子模块、PGO 等平台能力 | 达到明确进入条件后独立立项 |

## 3. 依赖关系

~~~mermaid
flowchart TD
    A["P0-00 文档与基线"] --> B["P0-01 生命周期内核"]
    B --> C["P0-02 状态机与管理端"]
    B --> D["P0-03 Module Registry"]
    D --> E["P0-04 SSRPC 与生成器"]
    B --> F["P0-05 配置快照与重载"]
    D --> G["P0-06 Bus 与按需 Driver"]
    B --> H["P0-07 Gateway Drain"]
    C --> I["P0-08 六服务迁移"]
    E --> I
    F --> I
    G --> I
    H --> I
    I --> J["P0-09 删除旧 API 与验收"]
    J --> K["P1 性能与工程治理"]
    K --> L["P2 平台增强"]
~~~

---

# P0：核心正确性

## P0-00：纳管文档并冻结基线

### 结果

- docs 不再被整体忽略。
- 当前代码能力和已知问题有可信快照。
- 测试与 benchmark 可用于重构前后比较。

### 文件

- 修改：.gitignore
- 修改：README.md
- 修改：CHANGELOG.md
- 修改：docs/STYLE.md
- 修改：docs/observability/README.md
- 归档：docs/architecture_review_2026-07.md
- 使用：docs/architecture_review_2026-07-v2.md
- 使用：docs/optimization_roadmap.md
- 创建：docs/benchmarks/baseline.md

### 任务

- [ ] 删除 .gitignore 中的 /docs/。
- [ ] 仅忽略 docs/benchmarks/raw、docs/profiles、pprof 和 trace 原始文件。
- [ ] 将旧评审移动到 docs/archive，并标记为优化前历史快照。
- [ ] README 只链接当前评审、执行计划、STYLE、SSRPC、可观测性和 benchmark 基线。
- [ ] 修正 observability 文档中“OTLP 尚未实现”的旧说明。
- [ ] 修正 CHANGELOG 对不存在文档的引用。
- [ ] 保存当前 Go 版本、GOOS、GOARCH、CPU、提交号和工作区状态。
- [ ] 运行全量测试、核心 Race 和 benchmark。
- [ ] 将汇总结果写入 docs/benchmarks/baseline.md。
- [ ] 原始结果放入忽略目录，不提交机器噪声。

### 命令

~~~powershell
git status --short
go version
go env GOOS GOARCH
go test -count=1 -timeout 600s ./lib/... ./src/... ./common/... ./module/... ./tools/protoc-gen-goone/... ./tools/cmd/...
go test -race -count=1 -timeout 300s ./lib/net/... ./lib/service/bootstrap/... ./lib/service/transaction/... ./lib/service/router/... ./lib/service/ssrpc/...
go test -run '^$' -bench . -benchmem -count=10 ./lib/api/sharedstruct/... ./lib/service/transaction/... ./lib/util/bufpool/...
~~~

### 当前参考值

| 项目 | 当前值 |
|---|---|
| Transaction | 634–680 ns/op，441–446 B/op，7–8 allocs |
| Serial key Transaction | 1587–1685 ns/op，471–479 B/op，8 allocs |
| SSPacket Header ToBytes | 3.52–3.59 ns/op，0 alloc |
| SSPacket Header To | 2.54–2.67 ns/op，0 alloc |
| CSPacket Header ToBytes | 12–16 ns/op，32 B/op，1 alloc |
| bufpool Get/Put | 19.5–20 ns/op，24 B/op，1 alloc |

### 验收

- [ ] git check-ignore docs/optimization_roadmap.md 无输出。
- [ ] README 中所有 docs 链接存在。
- [ ] 文档使用 UTF-8。
- [ ] 基线命令、环境和结果均可追溯。

---

## P0-01：统一 App 与 Component 生命周期

### 结果

删除 application.Init/Run 和 Hook 生命周期，所有运行资源都由 Component 管理。

### 文件

- 重构：lib/service/bootstrap/app.go
- 创建：lib/service/bootstrap/component.go
- 创建：lib/service/bootstrap/run.go
- 创建：lib/service/bootstrap/signal.go
- 创建：lib/service/bootstrap/signal_unix.go
- 创建：lib/service/bootstrap/signal_windows.go
- 创建：lib/service/bootstrap/lifecycle_test.go
- 最终删除：lib/service/application/

### 公共接口

~~~go
type Component interface {
    Name() string
    Start(context.Context) error
    Stop(context.Context) error
}

type Quiescer interface {
    Quiesce(context.Context) error
}

type Drainer interface {
    Drain(context.Context) error
}
~~~

### Component 契约

- [ ] Name 在 App 内唯一。
- [ ] Start 成功返回时组件已经可用。
- [ ] Start 失败时组件自行清理本组件的部分初始化。
- [ ] App 只把 Start 成功的组件加入 started stack。
- [ ] Stop 幂等。
- [ ] Stop 遵守 context。
- [ ] Stop 等待本组件 goroutine 退出。
- [ ] Stop 错误使用 errors.Join 汇总。
- [ ] 禁止组件内部 os.Exit、Fatal、Fatalf。
- [ ] 同一个 App 第二次 Run 返回明确错误。

### App.Run 算法

1. 校验 service name、options、timeout。
2. 加载初始配置快照。
3. 调用 Module.Register。
4. Seal Component、SSRPC、Driver Registry。
5. 状态进入 Starting。
6. 按注册顺序 Start。
7. 全部成功后进入 Ready。
8. 等待 parent context、SIGINT 或 SIGTERM。
9. 状态进入 Draining，readyz 立即失败。
10. 按反向顺序 Quiesce。
11. 使用 drain_timeout 按反向顺序 Drain。
12. Drain 完成或超时后进入 Stopping。
13. 使用 stop_timeout 逆序 Stop。
14. 进入 Stopped。
15. 返回运行、Drain、Stop 的聚合错误。

### 信号

- [ ] SIGINT/SIGTERM 启动正常排空。
- [ ] SIGUSR1 触发配置重载。
- [ ] 第二次 SIGINT/SIGTERM 取消 Drain，直接 Stop。
- [ ] Windows 与 Unix signal 使用 build tag 隔离。
- [ ] Run 返回前调用 signal.Stop。
- [ ] signal channel 有界，避免重复信号阻塞。

### 测试

- [ ] TestAppStartsComponentsInOrder。
- [ ] TestAppStopsComponentsInReverseOrder。
- [ ] TestAppRollsBackStartedComponentsOnStartFailure。
- [ ] TestAppDoesNotStopFailedComponent。
- [ ] TestAppJoinsStopErrors。
- [ ] TestAppStopIsIdempotent。
- [ ] TestAppDrainTimeoutContinuesToStop。
- [ ] TestAppSecondSignalCancelsDrain。
- [ ] TestAppRejectsDuplicateRun。
- [ ] TestComponentGoroutinesExitAfterRun。

### 验收

~~~powershell
go test -race -count=1 ./lib/service/bootstrap/...
~~~

### 注意

- 不用 sleep 判断组件是否启动。
- Start listener 必须等待 bind 成功或明确失败。
- 不在 Drain 中调用 Stop。
- 组件顺序使用 slice，不依赖 map。
- Admin、Logger、Tracing 应早启动、晚停止。
- Logger Flush 由日志组件 Stop 负责。

---

## P0-02：Ready、Allocated、Draining 状态机

### 文件

- 创建：lib/service/bootstrap/state.go
- 创建：lib/service/bootstrap/state_observer.go
- 修改：lib/service/bootstrap/admin.go
- 创建：lib/service/bootstrap/state_test.go
- 创建：lib/service/bootstrap/admin_test.go

### 状态

~~~go
type State string

const (
    StateStarting  State = "starting"
    StateReady     State = "ready"
    StateAllocated State = "allocated"
    StateDraining  State = "draining"
    StateStopping  State = "stopping"
    StateStopped   State = "stopped"
    StateFailed    State = "failed"
)
~~~

### 合法转换

| 当前 | 目标 |
|---|---|
| Starting | Ready、Failed |
| Ready | Allocated、Draining、Failed |
| Allocated | Draining、Failed |
| Draining | Stopping |
| Stopping | Stopped、Failed |
| Stopped | 无 |
| Failed | 无 |

### Observer

~~~go
type StateChange struct {
    Previous State
    Current  State
    At       time.Time
    Reason   string
    Deadline time.Time
    Metadata map[string]string
}

type StateObserver interface {
    OnStateChange(context.Context, StateChange) error
}
~~~

- [ ] Ready/Allocated Observer 失败会阻止状态提交。
- [ ] Draining/Stopping/Stopped Observer 失败只记录，不能阻止退出。
- [ ] Metadata 写入和读取都复制。
- [ ] 本轮不导入 Agones SDK。

### 管理端点

| 状态 | healthz | readyz |
|---|---:|---:|
| Starting | 200 | 503 |
| Ready | 200 | 200 |
| Allocated | 200 | 200 |
| Draining | 200 | 503 |
| Stopping | 503 | 503 |
| Stopped | 503 | 503 |
| Failed | 503 | 503 |

新增 GET /statez：

~~~json
{
  "service": "mainsvr",
  "state": "draining",
  "since": "2026-07-16T10:00:00Z",
  "reason": "SIGTERM",
  "drain_deadline": "2026-07-16T10:00:30Z",
  "allocated": false
}
~~~

完善 GET /components：name、state、ready、started_at、stopped_at、start_duration_ms、last_error。

### 安全

- [ ] admin IP 为空时默认 127.0.0.1。
- [ ] pprof 只挂载在 admin server。
- [ ] pprof 同时要求 admin.enabled 和 debug.pprof。
- [ ] 不增加未认证的 Allocate/Drain HTTP 写接口。
- [ ] statez 不输出配置和连接凭据。

### 测试

- [ ] 每个合法转换。
- [ ] 每个非法转换返回 ErrInvalidStateTransition。
- [ ] Draining 后 readyz 在等待资源前立即失败。
- [ ] StateObserver Ready 失败触发启动回滚。
- [ ] StateObserver Stop 失败不阻止退出。

---

## P0-03：显式 Module 与 Registry

### 文件

- 创建：lib/service/bootstrap/module.go
- 创建：lib/service/bootstrap/registry.go
- 创建：lib/service/bootstrap/registry_test.go
- 重构：lib/service/bootstrap/busapp/busapp.go

### 接口

~~~go
type Module interface {
    Name() string
    Register(*Registry) error
}
~~~

### 目标装配

~~~go
func NewApp() *bootstrap.App {
    cfg := appconfig.NewStore(
        gconf.LoadMainConfig,
        gconf.MergeMainReload,
    )

    return bootstrap.New(
        "mainsvr",
        bootstrap.WithConfig(cfg),
        bootstrap.WithModules(
            busapp.New(cfg, rabbitmq.Driver()),
            newMainModule(cfg),
        ),
    )
}
~~~

### Registry 规则

- [ ] Module 名称重复返回错误。
- [ ] Component 名称重复返回错误。
- [ ] nil Module/Component 返回错误。
- [ ] Module Register 失败时不启动任何组件。
- [ ] 错误包含 Module 名称和原始 error。
- [ ] Seal 后注册返回 ErrRegistrySealed。
- [ ] Seal 幂等，不重复构建内部对象。
- [ ] 注册顺序决定 Start 顺序。
- [ ] 反向顺序用于 Quiesce、Drain、Stop。

### 边界

- Module.Register 只能装配，不连接 Redis、Bus、DB，不启动 goroutine。
- 不引入 Fx/Wire 或反射 DI。
- Registry 不作为业务 Service Locator。
- 业务对象使用构造注入。
- 禁止 func init 注册 Module。

### 测试

- [ ] TestRegistryRejectsDuplicateModule。
- [ ] TestRegistryRejectsDuplicateComponent。
- [ ] TestRegistryRejectsNilComponent。
- [ ] TestRegistryRejectsRegistrationAfterSeal。
- [ ] TestRegistryPreservesOrder。
- [ ] TestRegistrationFailureStartsNothing。

---

## P0-04：统一 SSRPC 注册与不可变 Dispatcher

### 文件

- 创建：lib/service/ssrpc/registry.go
- 重构：lib/service/ssrpc/dispatcher.go
- 修改：lib/service/transaction/transaction_i.go
- 修改：lib/service/transaction/transaction_mgr_impl.go
- 修改：tools/protoc-gen-goone/generate.go
- 修改：tools/protoc-gen-goone/generate_test.go
- 重新生成：api/gen/**

### 删除 API

~~~go
RegisterMainC2SServiceToDispatcher(...)
RegisterMainC2SServiceToTransactionMgr(...)
~~~

### 新 API

~~~go
func RegisterMainC2SService(
    registry *ssrpc.Registry,
    server MainC2SServiceSServer,
) error
~~~

### Binding

~~~go
type BindingKind uint8

const (
    BindingCMD BindingKind = iota + 1
    BindingHTTP
    BindingWS
    BindingGRPCUnary
    BindingGRPCStream
)

func (r *Registry) Register(service string, bindings ...Binding) error
func (r *Registry) Seal() (*Dispatcher, error)
~~~

### 批次原子注册

1. 校验 service。
2. 校验全部 Binding。
3. 检查批次内部冲突。
4. 检查与已提交 Binding 冲突。
5. 全部通过后一次性提交。
6. 任意错误时不留下部分 Handler。

### 唯一 Key

- CMD：命令整数。
- HTTP：大写 Method + 规范 Path。
- WS：uint32 cmd。
- gRPC：full service name + method。

### Dispatcher

- [ ] Seal 后 CMD/HTTP/WS 使用只读 map。
- [ ] gRPC 在 Seal 时预分组。
- [ ] Dispatch 不再使用 RWMutex。
- [ ] Dispatcher 不暴露 Register 方法。
- [ ] Mount 每类 Transport 只执行一次。
- [ ] 未注册 Handler 返回 not-found，不 panic。

### TransactionMgr

RegisterCmd 改为返回 error：

- nil Handler 返回错误。
- 重复 CMD 返回错误。
- 已 Start 返回错误。
- 已 Closing 返回错误。
- 禁止 Fatal。

### 测试

- [ ] 重复 CMD 保留第一个 Handler。
- [ ] 重复 HTTP/WS/gRPC 返回错误。
- [ ] 批次内一个冲突导致整批不提交。
- [ ] Seal 后注册失败。
- [ ] 并发 Dispatch 通过 Race。
- [ ] TransactionMgr 启动后注册返回错误。
- [ ] 生成代码只出现统一 RegisterService。
- [ ] 生成代码传播注册错误。

### 命令

~~~powershell
go test -count=1 ./tools/protoc-gen-goone/...
go run ./tools/cmd/genproto
.\scripts\check_genproto.ps1 -Full
go test -race -count=1 ./lib/service/ssrpc/... ./lib/service/transaction/...
~~~

### 注意

- 不手改 api/gen/**。
- 不改变现有 CMD、HTTP path、WS cmd、gRPC full method。
- 不在 Dispatch 热路径拼接字符串。
- Seal 必须发生在 Bus consumer 和 listener Start 前。

---

## P0-05：不可变配置与安全局部重载

### 文件

- 创建：lib/service/appconfig/store.go
- 创建：lib/service/appconfig/reload.go
- 创建：lib/service/appconfig/store_test.go
- 拆分：common/gconf/config.go
- 创建：common/gconf/loaders.go
- 创建：common/gconf/reload_policy.go
- 更新所有 gconf.XxxSvrCfg 调用。

### Loader

由：

~~~go
func LoadMainConfig(path string) error
~~~

改为：

~~~go
func LoadMainConfig(
    context.Context,
    string,
) (*MainSvrConfig, error)
~~~

### Store

~~~go
type Loader[T any] func(context.Context) (*T, error)

type Merger[T any] func(
    oldConfig *T,
    candidate *T,
) (
    effective *T,
    restartRequired []string,
    err error,
)

type Store[T any] struct {
    current atomic.Pointer[T]
    loader  Loader[T]
    merger  Merger[T]
}
~~~

### 快照规则

- [ ] Loader 每次创建全新对象。
- [ ] 发布后的对象只读。
- [ ] map/slice 在合并时深拷贝。
- [ ] Current 不加互斥锁。
- [ ] 测试通过注入配置，不直接修改全局变量。

### 允许热更新

- base_cfg.debug.log_level。
- base_cfg.runtime.tracing.enabled。
- tracing.exporter。
- tracing.endpoint。
- tracing.insecure。
- tracing.sampler_ratio。
- tracing.headers。

### 必须重启

- Service identity、BusID。
- MQ 类型和地址。
- Registry 地址。
- TCP/WS/KCP/HTTP/gRPC/Admin 地址和端口。
- pprof。
- Redis、数据库、ORM。
- Gamedata 路径。
- Nacos。
- Shard 数、队列容量。
- Sign、REST 配置。
- 未列入白名单的其他字段。

### Reload 提交流程

1. 读取候选文件。
2. 解析到全新对象。
3. 完整校验。
4. 预构建候选 Trace Provider。
5. 预构建失败则关闭候选资源并保持旧状态。
6. 深拷贝旧快照形成 effective。
7. 合入白名单字段。
8. 收集其他差异为 restart_required。
9. 更新不可失败的 logger level。
10. 原子交换 effective 和 Trace Provider。
11. 使用短超时关闭旧 Provider。
12. 记录 applied 和 restart_required 字段名，不记录值。

### 测试

- [ ] 初始 Load 失败时无快照。
- [ ] 非法 Reload 不改变旧快照。
- [ ] log level 生效。
- [ ] tracing 先构建后交换。
- [ ] tracing 构建失败时 logger/config 不变。
- [ ] 修改端口只报告 restart_required。
- [ ] map/slice 不污染旧快照。
- [ ] 并发 Current/Reload 通过 Race。

### 注意

- 不使用完整配置百分号加 v 日志。
- Headers、DSN、密码、Token、Secret 脱敏。
- 不允许出现 logger 新、tracing 旧、config 新的部分提交。
- 本轮不热更新 DB、Bus、listener。

---

## P0-06：Bus、Router 与按需 Driver

### 文件

- 重构：lib/service/bootstrap/busapp/busapp.go
- 创建：lib/service/bus/registry.go
- 修改：lib/service/bus/bus.go
- 修改：lib/service/bus/driver/*/
- 删除生产引用：lib/service/bus/driver/all
- 重构：lib/service/router/router.go

### Driver

~~~go
type Driver struct {
    Name string
    New  BusCtor
}
~~~

各 Driver 包导出：

~~~go
func Driver() bus.Driver
~~~

应用显式装配：

~~~go
busapp.New(
    cfg,
    rabbitmq.Driver(),
)
~~~

### 规则

- [ ] 删除 Driver 注册型 func init。
- [ ] Driver Registry 为 App 实例级。
- [ ] 重复 Driver 返回错误。
- [ ] 未链接 Driver 启动失败并列出已注册名称。
- [ ] 错误不打印 MQ 密码。
- [ ] 默认只链接 RabbitMQ。
- [ ] 本轮不拆分多个 Go module。

### Router

- [ ] busapp 不再依赖 package-global Router。
- [ ] 使用 router.New。
- [ ] 将现有 Send、SelfBusID 调用改为小接口构造注入。
- [ ] 不把整个 App/Registry 注入业务层。

### Drain

1. Router 从服务注册中心注销。
2. admission gate 拒绝新请求。
3. 仍接收 DstTransID 非零的事务响应。
4. TransactionMgr 等待在途事务。
5. 停止 consumer。
6. 关闭 producer、watcher 和连接。

### 测试

- [ ] 只注册 RabbitMQ 时其他 Driver 不可创建。
- [ ] 未链接 Driver 返回明确错误。
- [ ] 重复 Driver 返回错误。
- [ ] Draining 后新请求拒绝。
- [ ] 在途响应仍能完成事务。
- [ ] Stop 后 watcher/consumer goroutine 退出。
- [ ] Start 失败触发生命周期回滚。

### 依赖检查

~~~powershell
go list -deps ./cmd/mainsvr | Select-String "nats.go|kafka-go|go-nsq|rocketmq"
~~~

默认 mainsvr 不应链接未选择 Driver。

### 注意

- Drain 开始时不能立即关闭整个 Bus。
- 新请求和事务响应在解析 Header 后分流。
- Registry 注销失败不阻止本地退出，但要记录错误和指标。

---

## P0-07：Gateway Quiesce、Session Drain 与 Stop

### 文件

- 重构：lib/net/net_mgr/net_i.go
- 创建：lib/net/net_mgr/lifecycle.go
- 创建：lib/net/net_mgr/session_tracker.go
- 修改：lib/net/tcp_server/tcp_server.go
- 修改：lib/net/ws_server/ws.go
- 修改：lib/net/ws_server/gin_ws.go
- 修改：lib/net/kcp_server/kcp.go
- 修改：lib/net/gnet_server/gnet_tcp.go
- 更新：src/connsvr/app.go

### 接口

~~~go
type GatewayServer interface {
    Name() string
    Start(context.Context) error
    Quiesce(context.Context) error
    Drain(context.Context) error
    Stop(context.Context) error

    ActiveConnections() int64
    ActiveSessions() int64

    SendByUID(uid uint64, header, body []byte) error
    BroadcastByZone(zone int32, header, body []byte)
    Kick(uid uint64, reason g1_protocol.EKickOutReason)
}
~~~

### 构造

删除 CreateTcpServer/CreateWebSocketServer/CreateKcpServer 式运行时初始化，改为构造阶段传入 Options，Start 只启动。

### 计数定义

- ActiveConnections：底层连接已建立且未关闭。
- ActiveSessions：已绑定 UID 的逻辑会话。
- 重复绑定不重复计数。
- OnClose 只减一次。
- 使用状态变更通知等待归零，不用轮询。

### Quiesce

- [ ] accepting=false。
- [ ] TCP/KCP 关闭 listener，保留已有连接。
- [ ] WS 停止新 Upgrade。
- [ ] gnet 无法暂停 listener 时在 OnOpened 拒绝。
- [ ] 已存在未认证连接不能再完成新登录绑定。
- [ ] readyz 已经返回 503。

### Drain

- [ ] 已有会话可以完成在途事务和主动退出。
- [ ] 等待 ActiveSessions 归零。
- [ ] 默认 drain_timeout=30s。
- [ ] 超时后进入 Stop。
- [ ] 记录起始会话、自然退出、强制关闭和耗时。

### Stop

- [ ] 关闭全部 listener/server。
- [ ] 关闭残留连接。
- [ ] 取消读写 goroutine。
- [ ] 等待全部 goroutine。
- [ ] 清空连接和 UID 索引。
- [ ] 幂等。

### 每种 Transport 测试

- [ ] Start 后可以连接。
- [ ] Quiesce 后新连接拒绝。
- [ ] 已有连接可以完成在途响应。
- [ ] 连接计数正确。
- [ ] 会话计数正确。
- [ ] 主动退出使 Drain 返回。
- [ ] 超时后 Stop 强制关闭。
- [ ] Stop 后端口可重新绑定。
- [ ] 并发 Send/Kick/Stop 无 Race。
- [ ] 无 goroutine 泄漏。

### 注意

- listener 必须保存为字段。
- HTTP Shutdown 超时后调用 Close。
- 不持有 session map 锁执行网络操作。
- Close 与 OnClose 并发时防止双减。
- 不假设异步写立即复制 buffer。

---

## P0-08：Task 调度器与六服务迁移

### 文件

- 创建：lib/service/scheduler/task.go
- 创建：lib/service/scheduler/task_test.go
- 修改：src/connsvr/app.go
- 修改：src/mainsvr/app.go
- 修改：src/infosvr/app.go
- 修改：src/mysqlsvr/app.go
- 修改：src/roomcentersvr/app.go
- 修改：src/web_svr/app.go
- 修改：cmd/*/main.go
- 修改：tools/cmd/scaffold/main.go

### Task

~~~go
type Task struct {
    TaskName   string
    Interval   time.Duration
    RunOnStart bool
    NonOverlap bool
    Run        func(context.Context) error
}
~~~

规则：

- [ ] 默认 NonOverlap=true。
- [ ] 使用可停止 timer/ticker。
- [ ] Stop 取消 context 并等待当前 Run。
- [ ] panic 记录并上报，不调用 Fatal。
- [ ] 不用 time.After 循环。
- [ ] 上一次未完成时跳过并增加 skipped 指标。

### connsvr

启动顺序：

1. Config。
2. Logger/Tracing/Admin。
3. Sign/REST。
4. SSRPC。
5. TransactionMgr。
6. Router/Bus。
7. TCP/WS/KCP。
8. Ready。

关停顺序：

1. Gateway Quiesce。
2. 等待 Session。
3. 排空 Transaction。
4. 停 Router/Bus。
5. 停 Gateway。
6. 停依赖。

### mainsvr

- [ ] Gamedata、Sensitive Words、Redis、ID Generator 组件化。
- [ ] Role manager 组件化。
- [ ] 角色 Task 每分钟执行。
- [ ] SelfLogoutSender 注入 Router/TransactionMgr。
- [ ] Transaction 排空后 FlushAllToDB。
- [ ] Flush 失败进入 Stop 聚合错误。

### infosvr

- [ ] Redis 组件化。
- [ ] Handler 注入 InfoMgr。
- [ ] 无通用 Tick。
- [ ] Transaction 后关闭 Redis。

### mysqlsvr

- [ ] OrmMgr 组件化。
- [ ] 新增 30 秒连接监控 Task。
- [ ] Tick(nowMs) 改为单次 MonitorConnections(ctx)。
- [ ] Transaction 后停止监控并关闭 ORM。
- [ ] 不依赖 OnExit 临时 Close。

### roomcentersvr

- [ ] Room manager 组件化。
- [ ] Room Tick 每 5 秒。
- [ ] Persist 每 10 秒。
- [ ] 删除 10ms 周期创建两个 goroutine。
- [ ] AI 初始化纳入 Component。
- [ ] Transaction 后 FlushAllToDB。

### web_svr

- [ ] HTTP、gRPC 分别实现 Component。
- [ ] HTTP Start 等待 bind。
- [ ] HTTP Drain 使用 Shutdown。
- [ ] gRPC Drain 使用 GracefulStop。
- [ ] 超时后 gRPC Stop。
- [ ] Reflection 显式配置。
- [ ] 移除 OnProc。
- [ ] listener Start 前完成 Route Mount。

### main.go

全部改为单一 App.Run，不再引用 lib/service/application。

### Scaffold

- [ ] 生成 cmd/service/main.go。
- [ ] 生成 src/service/app.go。
- [ ] 生成 Module。
- [ ] 使用 IDL RegisterService。
- [ ] 不生成 cmd_handler.RegCmd。
- [ ] 不生成 OnProc/OnTick。
- [ ] 不默认导入全部 Driver。
- [ ] 生成最小生命周期测试。

### 验收

~~~powershell
go test -race -count=1 ./lib/service/bootstrap/... ./lib/service/scheduler/... ./src/...
.\build.ps1
~~~

---

## P0-09：删除旧 API 与总验收

### 删除清单

- [ ] lib/service/application。
- [ ] bootstrap.ServiceApp。
- [ ] bootstrap Hook Options。
- [ ] OnInit、OnReload、OnProc、OnTick、OnExit。
- [ ] RegisterToDispatcher 双轨。
- [ ] RegisterToTransactionMgr 双轨。
- [ ] driver/all 生产引用。
- [ ] Driver 注册型 init。
- [ ] package-global 可变配置。
- [ ] package-global default Router 的生产调用。
- [ ] runtime.GOMAXPROCS。
- [ ] 完整配置日志。

### 静态扫描

~~~powershell
rg -n "application\.(Init|Run)|NewServiceApp|OnProc|OnTick" cmd src lib tools
rg -n "Register.*ToDispatcher|Register.*ToTransactionMgr" api/gen tools/protoc-gen-goone
rg -n "driver/all|runtime\.GOMAXPROCS" cmd src lib
rg -n "gconf\.(ConnSvrCfg|MainSvrCfg|InfoSvrCfg|MySqlSvrCfg|RoomCenterSvrCfg|WebSvrCfg)" src lib module
~~~

生产代码不应再有命中。

### 完整验证

~~~powershell
go build ./...
go vet -composites=false ./...
go test -count=1 -timeout 600s ./lib/... ./src/... ./common/... ./module/... ./tools/protoc-gen-goone/... ./tools/cmd/...
go test -race -count=1 -timeout 300s ./lib/net/... ./lib/service/bootstrap/... ./lib/service/scheduler/... ./lib/service/appconfig/... ./lib/service/transaction/... ./lib/service/router/... ./lib/service/ssrpc/...
.\scripts\check_genproto.ps1 -Full
.\build.ps1
~~~

### 服务冒烟

每个服务验证：

- [ ] 正常启动进入 Ready。
- [ ] 配置非法时退出码非零。
- [ ] 中间组件 Start 失败时完整回滚。
- [ ] SIGUSR1 更新日志级别。
- [ ] 端口修改只报告 restart_required。
- [ ] SIGTERM 立即使 readyz 失败。
- [ ] Drain 后进程退出。
- [ ] Drain 超时后强制退出。
- [ ] 第二次 SIGTERM 立即 Stop。
- [ ] 退出后端口释放。
- [ ] 日志无敏感配置。

### 发布

1. P0 在同一功能分支完成。
2. 每个提交保持可构建。
3. P0 完成前不部署中间生命周期。
4. 先灰度 web_svr、infosvr。
5. 再灰度 mysqlsvr、mainsvr、roomcentersvr。
6. 最后灰度 connsvr。
7. 一次只替换一个实例。
8. 观察 Ready、Drain、错误率、Session、Transaction。
9. 回滚旧二进制和旧配置，不涉及数据迁移。

---

# P1：性能和工程治理

## P1-01：Buffer Lease 与 Packet 编码

### 文件

- 重构：lib/util/bufpool/bufpool.go
- 创建：lib/util/bufpool/bufpool_bench_test.go
- 修改 TCP/WS/KCP 写队列。
- 修改 CSPacket 热路径。
- 新增 Gateway frame benchmark。

### API

~~~go
type Buffer struct {
    Bytes []byte
}

func Acquire(size int) *Buffer
func Release(*Buffer)
~~~

### Ownership

- [ ] Acquire 后调用方拥有 Lease。
- [ ] 入队成功后 writer 拥有。
- [ ] 入队失败 sender 释放。
- [ ] writer 完成、错误、关闭都只释放一次。
- [ ] Stop 释放队列残留。
- [ ] Release 后不得引用 Bytes。
- [ ] 超过 64 KiB 不进入 Pool。
- [ ] 异步 API 未确认复制前不能释放。

### 写队列

由 chan []byte + nil 控制信号改为：

- chan *bufpool.Buffer 传数据。
- context 或单独 close channel 控制关闭。

### Header

生产热路径使用栈上数组和 To(dst)，不使用 ToBytes 后再复制。

### 目标

- Buffer Acquire/Release 稳态 0 alloc。
- Gateway Header 准备 0 alloc。
- Transaction 中位数不回退超过 5%。
- benchmark 至少 10 次。

### 注意

gnet v1 仍在 adapter 内复制，不能错误复用异步写内存。

---

## P1-02：性能基准、Profile 与决策门禁

### 新 Benchmark

- [ ] Dispatcher CMD lookup。
- [ ] Dispatcher WS lookup。
- [ ] Transaction 无下游调用。
- [ ] Transaction 一次下游响应。
- [ ] 同 UID serial queue。
- [ ] 不同 UID 并发。
- [ ] Scheduler idle。
- [ ] Scheduler non-overlap。
- [ ] Gateway encode/enqueue。

### 采集

- ns/op。
- B/op。
- allocs/op。
- goroutine。
- CPU profile。
- heap profile。
- mutex/block profile。
- p50/p95/p99。

### Transaction 改造进入条件

满足任一条件才改变 Transaction goroutine 模型：

- goroutine 创建超过 CPU 10%。
- 栈内存成为容量上限。
- Transaction 分配超过总分配 20%。
- 达到业务 QPS 前出现 scheduler 瓶颈。

否则保留在途事务 goroutine 和 serial key。

### 验收

- [ ] 空闲服务无 application 100Hz Tick。
- [ ] roomcenter 无每 10ms 双 goroutine。
- [ ] Dispatcher mutex profile 无查找锁。
- [ ] 核心吞吐无超过 5% 稳定回退。

---

## P1-03：可观测性与安全

### 指标

- goone_lifecycle_state。
- goone_component_start_duration_seconds。
- goone_component_start_failures_total。
- goone_drain_duration_seconds。
- goone_drain_timeouts_total。
- goone_active_connections。
- goone_active_sessions。
- goone_forced_session_closes_total。
- goone_task_duration_seconds。
- goone_task_skipped_total。
- goone_config_reload_total。

### 日志事件

- component_starting。
- component_started。
- component_start_failed。
- state_changed。
- drain_started。
- drain_completed。
- drain_timed_out。
- component_stop_failed。
- config_reloaded。
- config_reload_failed。
- config_restart_required。

### 安全

- [ ] 删除完整配置日志。
- [ ] DSN、Password、Token、Secret、Headers 脱敏。
- [ ] pprof 仅 admin。
- [ ] gRPC Reflection 默认关闭。
- [ ] admin 默认 loopback。
- [ ] statez/components 不暴露连接凭据。

### Dashboard

更新 docs/observability：

- 生命周期状态。
- Drain 时间。
- 活跃连接/会话。
- 强制断线。
- Reload 成功/失败。
- Component 启动失败。

---

## P1-04：STYLE、Scaffold 与 CI

### STYLE

- [ ] Runtime 能力实现 Component。
- [ ] 禁止新增生命周期 Hook。
- [ ] 禁止 init 注册 Driver/Handler/Module。
- [ ] 注册必须返回 error。
- [ ] 重复注册必须失败。
- [ ] 配置发布后不可修改。
- [ ] Reload 必须白名单。
- [ ] goroutine 必须归属于可停止组件。
- [ ] Start 完成后才能 Ready。
- [ ] Draining 先于 Stop。
- [ ] 性能修改附 benchmark。
- [ ] 异步 Buffer 写明 Ownership。
- [ ] 框架接口不使用 I 前缀。
- [ ] 导出符号使用英文 Godoc。

### CI

- [ ] 移除 lint continue-on-error。
- [ ] 保留 new-from-rev 冻结存量债务。
- [ ] Race 加入 bootstrap、scheduler、appconfig。
- [ ] 增加 docs link check。
- [ ] 增加旧生命周期静态扫描。
- [ ] check-genproto 继续阻止漂移。

### 验收

~~~powershell
golangci-lint run --new-from-rev=origin/dev
go test -race ./lib/service/bootstrap/... ./lib/service/scheduler/... ./lib/service/appconfig/...
~~~

---

## P1-05：回归与压力测试

### 矩阵

| 场景 | 连接数 | 行为 |
|---|---:|---|
| Idle TCP | 1,000 / 10,000 | 长连接无请求 |
| Login burst | 1,000 / 5,000 | 并发登录绑定 |
| Mixed packets | 1,000 / 10,000 | 心跳、查询、写混合 |
| Serial UID | 1 / 100 | 同 UID 高频顺序请求 |
| Multi UID | 1,000+ | 不同 UID 并发 |
| Drain | 1,000 / 10,000 | 压测中 SIGTERM |
| Reload | 稳定流量 | 多次 SIGUSR1 |
| Dependency failure | 稳定流量 | Redis/MQ 短暂故障 |

### 观察

- 错误率。
- p50/p95/p99。
- RSS、Heap、Goroutine。
- GC Pause、GC CPU。
- Transaction active/pending/dropped。
- Drain duration。
- 自然退出/强制关闭会话。
- Bus late response。
- Reload 后稳定性。

### 验收

- [ ] 无 Race。
- [ ] 无 Panic/Fatal。
- [ ] Session/Connection 计数不为负。
- [ ] Drain 在最大时间内退出。
- [ ] 端口释放。
- [ ] 核心吞吐无超过 5% 回退。
- [ ] 重复 Drain/Reload 不泄漏资源。

---

# P2：平台级增强

## P2-01：gnet v2 与 Writev

### 进入条件

满足任一项：

- gnet 写复制超过 CPU 10%。
- 目标超过 50,000 长连接/实例。
- Gateway write allocation 超过总分配 15%。
- gnet v1 维护或安全风险不可接受。

### 方案

- [ ] 升级 gnet v2。
- [ ] 使用 AsyncWritev 或等价分段写。
- [ ] Header/Body 不合并复制。
- [ ] 完成回调释放 Buffer Lease。
- [ ] 连接关闭处理未完成写。
- [ ] gonet 保留为正确性对照和回滚后端。

### 验收

- [ ] gnet 写路径不创建 merged slice。
- [ ] 无 double release/use-after-release。
- [ ] 吞吐提升至少 10%，或分配下降至少 20%。
- [ ] 未达到收益阈值则不合并复杂化方案。

---

## P2-02：Agones 适配

### 进入条件

- 出现独立可分配的专用游戏服。
- 正式采用 Kubernetes。
- Fleet、Allocator、扩缩容策略确定。

当前六个共享服务不直接视为单局 Allocated 实例。

### 文件规划

- 创建：lib/contrib/agones/adapter.go
- 创建：lib/contrib/agones/health.go
- 创建：deploy/k8s/base/
- 创建：deploy/k8s/overlays/dev/
- 创建：deploy/k8s/overlays/prod/
- 创建：docs/deployment/agones.md

### 映射

| GoOne | Agones |
|---|---|
| Starting | 不 Ready |
| Ready | SDK Ready |
| Allocated | SDK Allocate 或外部分配确认 |
| Draining | GoOne 内部停止接流与排空 |
| Stopping | SDK Shutdown |
| Stopped | Pod 退出 |
| Failed | 停止 Health，触发替换 |

### 验收

- [ ] 未 Ready 不进入分配池。
- [ ] Allocate 状态同步。
- [ ] SIGTERM、缩容、SDK Shutdown 进入同一 Drain。
- [ ] Drain 超时仍退出。
- [ ] Agones SDK 只在适配包，不污染 bootstrap。

---

## P2-03：MQ Driver 独立 Go Module

### 进入条件

- 未使用 Driver 明显影响依赖下载或漏洞扫描。
- Driver 需要独立版本。
- 显式装配稳定至少一个版本。

### 结构

~~~text
contrib/bus/rabbitmq
contrib/bus/nats
contrib/bus/kafka
contrib/bus/nsq
contrib/bus/rocketmq
~~~

### 要求

- [ ] 核心 Bus 不依赖具体 MQ SDK。
- [ ] 每个 Driver 只依赖核心接口和自身 SDK。
- [ ] 独立 tag。
- [ ] 主仓默认只 require RabbitMQ。
- [ ] 每个 module 单独 build/test。
- [ ] 不使用开发者本机 replace。

### 验收

- [ ] 核心 go.mod 不含未使用 MQ SDK。
- [ ] 单 Driver 可独立升级。
- [ ] Driver contract test 一致。
- [ ] 主服务行为不变。

---

## P2-04：PGO、故障注入与容量模型

### PGO 进入条件

- 有稳定、接近生产的压力场景。
- Profile 覆盖至少 20–30 分钟代表流量。
- Profile 无 debug 日志和测试工具污染。

### PGO

- [ ] connsvr、mainsvr、roomcentersvr 分别采集。
- [ ] 不共用不具代表性的 Profile。
- [ ] 代表性基准提升至少 3%。
- [ ] 关键场景无超过 2% 回退。
- [ ] Go 大版本升级后重新验证。

### 故障注入

- [ ] MQ 短断与恢复。
- [ ] Registry 不可达。
- [ ] Redis 慢响应。
- [ ] Tracing Exporter 不可达。
- [ ] Drain 中依赖失败。
- [ ] Stop 超时。
- [ ] 第二次终止信号。
- [ ] 大量客户端同时断开。

### 容量文档

创建 docs/capacity/，记录：

- 单实例连接数。
- 稳定 QPS。
- Transaction shard 建议值。
- 每 UID pending 上限。
- 内存/连接。
- Drain 时间与连接数关系。
- 扩缩容安全阈值。

---

# 4. 推荐提交顺序

1. docs: track review and benchmark baseline
2. refactor: add component lifecycle and state machine
3. refactor: add module and immutable ssrpc registry
4. refactor: add immutable config and safe reload
5. refactor: make bus drivers explicit components
6. refactor: add gateway quiesce and drain
7. refactor: migrate services to unified app
8. refactor: remove legacy lifecycle and registration
9. perf: reduce gateway buffer allocations
10. chore: enforce lifecycle style and CI gates

每个提交必须可编译并通过对应测试，不混入无关全仓格式化。

# 5. 全程注意事项

- 不手工修改 api/gen、protobuf 和 gamedata 生成文件。
- 保留工作区现有 common（原 game_protocol）修改和 .zcode，不覆盖、不清理。
- 先写失败测试，再修改实现。
- 不用 sleep 模拟生命周期同步。
- 不持锁执行 callback、网络写、Drain 或 Stop。
- 不用 package-global map 注册 Driver、Handler、Module。
- 最终不保留 deprecated wrapper。
- P0 不升级 gnet v2、不拆多 module、不引入 Agones。
- 不把所有 goroutine 当成性能问题，Transaction 以 Profile 决策。
- 不复用仍被异步写持有的 Buffer。
- Draining 阶段 readyz 必须失败。
- 某组件 Stop 失败不得跳过其他组件。
- Reload 不修改 listener、MQ、DB 等静态资源。
- 不打印完整配置和敏感字段。
- 不使用 Fatal 代替错误返回。
- 定时任务必须支持取消、等待、禁止重入。
- 每个 P0 子任务完成后运行对应 Race。
- 每个性能修改更新 benchmark 文档。

# 6. 最终完成标准

- [x] 六个服务全部使用单一 App.Run。
- [x] Runtime 资源由 Component 管理。
- [x] 启动失败完整逆序回滚。
- [x] Ready、Allocated、Draining 行为明确。
- [x] Gateway、Bus、Transaction、HTTP、gRPC 可排空。
- [x] Drain 和 Stop 都有超时。
- [x] SIGUSR1 使用不可变快照（appconfig.Store 原语就绪；六服务接入待 gconf 迁移）。
- [x] 重复 Handler 在启动期失败（ssrpc.Registry 批量原子注册）。
- [x] Dispatcher 热路径无注册锁（Seal 后只读 map，2.84ns/0alloc）。
- [x] 默认二进制只链接显式 Driver（方案 B P1-04：5 个 bus 服务显式 DriverRegistry+MustRegister(rabbitmq)，bussvc 不再 blank-import driver/all，websvr 不链接任何 MQ SDK）。
- [x] 无通用 10ms Tick/Proc。
- [x] 不覆盖 Runtime GOMAXPROCS。
- [x] P0 build/test/race/genproto 全通过。
- [x] P1 benchmark 无不可解释回退。
- [x] STYLE、Scaffold、README、评审、计划和代码一致。

---

## 7. 方案 B 闭环（2026-07 实施）

方案 B 在 roadmap P0+P1 基础上完成了生产闭环。后续证据与容量矩阵由 v3 计划维护：见 [modernization_execution_plan_2026-07-v3.md](modernization_execution_plan_2026-07-v3.md) 与 [architecture_review_2026-07-v3.md](architecture_review_2026-07-v3.md)；容量矩阵 `benchmarks/capacity-matrix.md` 待 V3-P1-06 产出。

新增完成项（超出原 roadmap）：

- [x] 终止信号单一 dispatcher + Drain 真实 DeadlineExceeded + ErrDrainEscalated（P0-01）。
- [x] 状态机为唯一事实源（删 App 重复字段、gauge 时机修复、Allocate(ctx)error）（P0-02）。
- [x] Admin 延迟取配置 + App 自持 Tracker + RuntimeErrorSource 监督（P0-03）。
- [x] 网络 deadline 用 time.Now()、写缓冲所有权、Stop 锁外关闭、WS 同步 Listen（P0-04）。
- [x] 共享 SessionHub（跨传输原子重绑、IPv6、锁外 I/O）+ SessionTracker.WaitSessions/CAS/Close（P0-05/06）。
- [x] gnet admission gate（P0-06，仅补 gate，backend 分派已正确）。
- [x] websvr 单 Dispatcher 共享 + Drain 超时保留强关路径（P0-07）。
- [x] Registry.Seal 真幂等 + RegisterBindings + RegistryComponent + TransMgr.RegisterCmdE（P1-01/02）。
- [x] generator 生成 `<Service>Bindings`/`RegisterToRegistry` + 6 服务迁移（P1-03）。
- [x] amqp091-go 迁移（streadway/amqp 从依赖图消失）（P1-05）。
- [x] appconfig.Store writeMu 串行化 + MergeResult.Applied 真填充（P1-06）。
- [x] runtime.MustNew + MustRegister 可变参数 + scaffold run()/-root/NewApp 修复（P1-07/08）。

待运维执行（P2-02/P2-03）：C2–C4 压测机采集、RabbitMQ/Redis 真实重启演练、gnet vs gonet 对比、兼容入口清理门禁（容量矩阵 `benchmarks/capacity-matrix.md` 待 V3-P1-06 产出）。

