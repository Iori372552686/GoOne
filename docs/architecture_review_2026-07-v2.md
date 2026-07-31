# GoOne 架构复审 v2：生命周期、模块化与性能治理

> 评审日期：2026-07-16  
> 评审范围：cmd/、src/、lib/、common/、module/、tools/、deploy/、docs/  
> 评审基线：当前工作区代码优先于 README 和历史文档  
> 配套执行计划：[optimization_roadmap.md](optimization_roadmap.md)

## 1. 执行摘要

GoOne 已经不是“只有基础骨架”的游戏后端。近期迭代已经补齐或显著改善了 IDL 驱动的 SSRPC、分片串行事务、Router 实例化、背压、链路追踪、gnet/KCP、持久化排空、配置分组、CI、Race 测试和压力测试工具。旧评审中“没有 CI、没有 buffer pool、gnet/KCP 不可用、没有 OTLP”等判断已经失效。

当前主要矛盾已经从“缺能力”转为“能力之间缺少统一生命周期和明确边界”：

| 维度 | 当前判断 | 主要差距 |
|---|---|---|
| 游戏领域模型 | 良好 | Transaction、Role、Room 的生命周期仍未完全纳入统一运行时 |
| RPC 与协议 | 良好 | 注册双轨、重复覆盖、Dispatcher 热路径锁 |
| 启停可靠性 | 需优先整改 | application/bootstrap 双生命周期，启动失败回滚不完整 |
| 滚动发布 | 需优先整改 | Gateway 无服务级 Quiesce/Drain/Stop，会话无法真正排空 |
| 配置治理 | 需优先整改 | SIGUSR1 原地修改包级全局配置，存在并发和部分生效风险 |
| 性能 | 中上 | 空闲轮询、短周期 goroutine、CSPacket 与 bufpool 仍有可测分配 |
| 组件按需加载 | 一般 | busapp 默认链接所有 MQ Driver |
| 可观测性 | 中上 | 缺少 Ready/Allocated/Draining 状态和排空指标 |
| 工程一致性 | 一般 | 文档被忽略、脚手架和 STYLE 与当前实现不一致 |

本轮不建议推翻 GoOne 的分片事务和游戏领域管理器。正确方向是保留差异化的游戏后端能力，用主流框架已经验证的工程方法统一其启动、注册、排空和观测：

- 采用 Kratos 风格的统一 Component Start/Stop。
- 采用 Pitaya 风格的显式 Module 注册与会话排空。
- 采用 due 风格的应用按需选择组件和 Driver。
- 采用 Agones 清晰的 Ready/Allocated/Shutdown 语义，并在 GoOne 内部增加 Draining 阶段。

## 2. 评审方法与事实来源

本次评审使用以下优先级判断事实：

1. 当前代码和测试。
2. 当前配置文件和部署脚本。
3. 当前 CI。
4. README、CHANGELOG 和 docs。
5. 历史架构评审。

当文档与代码冲突时，以代码为准，并在文档整改任务中修正冲突。

重点检查入口：

- 服务启动：cmd/*/main.go、src/*/app.go。
- 公共启动层：lib/service/application、lib/service/bootstrap。
- RPC 注册：lib/service/ssrpc、tools/protoc-gen-goone。
- 事务模型：lib/service/transaction。
- Bus 与 Router：lib/service/bus、lib/service/router。
- Gateway：lib/net/net_mgr、tcp_server、ws_server、kcp_server、gnet_server。
- 配置：common/gconf。
- 发布：deploy/scripts/server.sh。

## 3. 当前架构与已完成能力

### 3.1 运行模型

当前活动服务包括：

- connsvr：TCP、WebSocket、KCP 客户端网关。
- mainsvr：玩家角色与核心业务。
- infosvr：轻量资料和缓存业务。
- mysqlsvr：数据库持久化业务。
- roomcentersvr：房间生命周期和周期任务。
- web_svr：HTTP 与可选 gRPC 接入。

总线服务的主要消息路径：

~~~text
client
  -> connsvr
  -> router / bus
  -> TransactionMgr shard
  -> SSRPC handler
  -> domain manager
~~~

TransactionMgr 根据 RouterID 或 UID 建立串行键，在保证单实体顺序的同时允许不同实体并发。这是 GoOne 最重要的游戏领域差异化之一，应保留并强化，而不是为了模仿通用微服务框架而移除。

### 3.2 近期已经完成的优化

当前代码已经具备：

- SSRPC 的 CMD、HTTP、WS、gRPC 多传输绑定。
- proto 生成器和生成代码一致性检查。
- TransactionMgr 分片、serial key、背压与 Close 排空。
- Router 结构体和服务注册/发现能力。
- RabbitMQ、NATS、Kafka、NSQ、RocketMQ Driver 分层。
- TCP、WS、KCP 和 gnet Gateway 路径。
- bufpool 和 Packet Header 无分配编码能力。
- Prometheus、Tracing 和 OTLP HTTP exporter。
- Role、Room 停机刷盘路径。
- GitHub Actions build、test、race、genproto、lint 作业。
- tester regression/stress 工具。

因此，本轮的重点不是重复实现这些能力，而是将它们放进统一、可验证的生命周期。

## 4. 与主流框架的差距

### 4.1 Kratos：统一 Start/Stop 生命周期

[Kratos transport](https://go-kratos.dev/docs/component/transport/overview/) 使用清晰的 Start(context.Context) 和 Stop(context.Context) 管理服务端组件。其价值不是接口名称本身，而是：

- 每个运行资源有明确所有者。
- 启动成功和“已创建 goroutine”不是同一件事。
- 关闭由 context 约束。
- App 可以统一组织启动和逆序停止。

GoOne 当前由 application 负责信号和循环，由 bootstrap 负责阶段 Hook。两层职责交叉，导致：

- 全局 application.app 限制多实例测试。
- application.Init 在错误时 Fatal/Exit。
- bootstrap 启动中途失败时只清理 admin。
- InitDeps、StartRuntime 内部启动了哪些资源无法由框架可靠追踪。
- OnShutdown、OnExit 的顺序依赖人为约定。

结论：合并为单一 App.Run，并将运行资源改为 Component。

### 4.2 Pitaya：Module 注册与 Session Drain

[Pitaya API](https://pitaya.readthedocs.io/en/latest/API.html) 将组件初始化、启动后处理和关闭前处理纳入组件生命周期；其会话排空机制会等待 Session Module 退出，并受超时约束。

GoOne 当前具备事务排空，但网关缺少：

- 停止接纳新连接。
- 服务级活跃连接和逻辑会话计数。
- 等待会话自然退出。
- 超时后关闭残留连接。
- 对 TCP、WS、KCP、gnet 一致的 Stop 语义。

结论：引入 Quiesce、Drain 两个可选生命周期接口；先停止接流，再等待会话和事务，最后强制 Stop。

### 4.3 due：按需组件

[due](https://github.com/dobyte/due) 的核心价值之一是应用按需选择网络、注册中心和集群组件，而不是核心默认加载所有实现。

GoOne 的 bus Driver 已经物理分包，但 busapp 仍通过 driver/all 默认链接所有实现，带来：

- 二进制包含未使用 SDK。
- 依赖升级和漏洞扫描面扩大。
- Driver 通过 init 注册，重复和顺序错误难以显式处理。
- 为减小二进制而绕过 busapp 会同时失去标准生命周期。

结论：Driver 导出显式描述符，由具体 App 选择；默认部署只链接 RabbitMQ。

### 4.4 Agones：明确的服务状态

[Agones Client SDK](https://agones.dev/site/docs/guides/client-sdks/) 提供 Ready、Allocate、Shutdown 等清晰动作。Agones 本身并没有替代业务排空的通用 Drain 方法。

GoOne 应采用：

~~~text
Starting -> Ready -> optional Allocated -> Draining -> Stopping -> Stopped
~~~

其中：

- Ready：实例可以接收新流量或新分配。
- Allocated：实例已分配给具体游戏或会话，仍可服务已有流量。
- Draining：GoOne 内部状态，停止接收新流量并等待存量工作。
- Stopping：排空完成或超时，执行强制关闭。
- Stopped：全部组件退出。

本轮只实现内部状态机与 StateObserver 适配点，不直接引入 Agones SDK 或 Kubernetes 清单。

## 5. P0 架构问题

### 5.1 application/bootstrap 双生命周期

当前所有 cmd 入口执行：

~~~go
application.Init(service.NewApp())
application.Run()
~~~

ServiceApp 又在 OnInit 中执行配置、日志、依赖、注册、Runtime 和 Ready。问题包括：

- Init 和 Run 是两个全局过程，调用顺序只能靠约定。
- 初始化失败直接影响进程退出，无法在测试中自然断言错误。
- App 不能在同一进程创建两个隔离实例。
- OnProc 和 OnTick 被所有服务共同承担，即使服务根本不需要。

目标入口：

~~~go
func main() {
    flag.Parse()

    if err := mainsvr.NewApp().Run(context.Background()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
~~~

### 5.2 启动失败回滚不完整

ServiceApp 的启动顺序已经阶段化，但 InitDeps、RegisterHandlers 或 StartRuntime 失败后，清理逻辑主要关闭 admin，无法逆序关闭：

- 已建立的 Tracing Provider。
- 已连接的 Redis/ORM。
- 已启动的 TransactionMgr shard。
- 已启动的 Router、Bus consumer 和 watcher。
- 已监听的 TCP/WS/KCP/HTTP/gRPC 端口。

目标规则：

- Start 成功后才进入 started stack。
- 后续 Start 失败时，只逆序 Stop 已成功组件。
- 返回 errors.Join 聚合错误。
- 不因一个 Stop 失败跳过其他组件。

### 5.3 GOMAXPROCS 覆盖 Go 1.25 行为

bootstrap 当前调用：

~~~go
runtime.GOMAXPROCS(runtime.NumCPU() + 1)
~~~

[Go 1.25 Release Notes](https://go.dev/doc/go1.25) 说明 Runtime 已能根据容器 CPU 限额设置并动态更新 GOMAXPROCS。应用主动调用 GOMAXPROCS 会关闭该动态行为。

此调用应直接删除。若未来存在特殊调优需求，应通过部署环境 GOMAXPROCS 或明确配置进行，并附容器压测证据。

### 5.4 注册静默覆盖

当前 Dispatcher 的 RegisterCmd、RegisterHTTP、RegisterWS、RegisterGRPC 会覆盖重复 key 或忽略无效参数，TransactionMgr.RegisterCmd 在启动后注册时调用 Fatal。

风险：

- 两个 proto 服务 CMD 冲突时，后注册者悄悄替换前者。
- HTTP path 冲突直到运行时才暴露异常。
- 生成代码同时提供 ToDispatcher 和 ToTransactionMgr，应用容易选择不同路径。
- Dispatcher 运行期仍使用 RWMutex 查找 handler。

目标：

- 统一 ssrpc.Registry。
- 批量注册全部成功后才提交。
- 重复 key 返回错误。
- Seal 后注册返回错误。
- Seal 生成不可变 Dispatcher。
- 生成器只生成 Register<Service>(registry, server) error。

### 5.5 配置重载不安全

当前 Loader 修改 gconf 包级结构体，SIGUSR1 再次调用 Loader 并重建 logger。运行中其他 goroutine 可能同时读取同一结构体。

风险：

- data race。
- map/slice 更新期间被并发读取。
- 配置展示为新端口，但 listener 仍使用旧端口。
- logger 已更新而 tracing 更新失败，产生部分提交。

目标：

- Loader 返回全新配置对象。
- 原子发布不可变快照。
- 热更新白名单仅包含日志级别和 Tracing。
- 网络、Bus、数据库、身份、容量等字段报告 restart_required。
- 新 Tracing Provider 预构建成功后再提交。

### 5.6 Gateway 无完整排空

TCP/KCP listener 当前只存在于启动函数局部变量中，WS HTTP Server 和 gnet 也没有统一 Quiesce/Drain 契约。

目标：

1. App 先进入 Draining，readyz 立即失败。
2. Gateway 设置 accepting=false。
3. 可关闭 listener 的实现停止 Accept。
4. 已有会话继续处理在途工作。
5. 等待 ActiveSessions 归零，默认 30 秒。
6. 超时后 Stop 关闭残留连接。
7. 第二次终止信号立即取消等待。

本轮不实现透明会话迁移。被强制关闭的客户端依赖现有重连机制恢复。

## 6. 目标架构

### 6.1 Component

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

约束：

- Start 成功返回时组件已经可以工作。
- Start 失败必须清理本组件部分资源。
- Stop 幂等、受 context 限制、等待所属 goroutine。
- 所有运行 goroutine 必须归属于某个 Component。
- 禁止 Component 内调用 Fatal 或 os.Exit。

### 6.2 Module 与 Registry

~~~go
type Module interface {
    Name() string
    Register(*Registry) error
}
~~~

Module 只描述装配，不得在 Register 中连接外部依赖或启动 goroutine。

Registry 负责：

- Module 唯一性。
- Component 唯一性。
- SSRPC Binding 唯一性。
- Driver 唯一性。
- 保持 Start 顺序。
- Seal 后禁止修改。

不引入反射型 DI 容器。依赖继续通过显式构造函数注入，保持 Go 代码可读、可测试。

### 6.3 App 运行顺序

~~~mermaid
sequenceDiagram
    participant Main
    participant App
    participant Config
    participant Registry
    participant Components
    participant Admin

    Main->>App: Run(ctx)
    App->>Config: Load immutable snapshot
    App->>Registry: Register modules
    App->>Registry: Seal
    App->>Components: Start in order
    App->>Admin: state = Ready
    Main-->>App: SIGTERM
    App->>Admin: state = Draining, ready=false
    App->>Components: Quiesce in reverse
    App->>Components: Drain in reverse
    App->>Admin: state = Stopping
    App->>Components: Stop in reverse
    App->>Admin: state = Stopped
~~~

### 6.4 状态与健康端点

| 状态 | healthz | readyz | 说明 |
|---|---:|---:|---|
| Starting | 200 | 503 | 进程存活，尚未接流 |
| Ready | 200 | 200 | 可以接流 |
| Allocated | 200 | 200 | 已分配，继续服务 |
| Draining | 200 | 503 | 存活但拒绝新流量 |
| Stopping | 503 | 503 | 强制关闭阶段 |
| Stopped | 503 | 503 | 已停止 |
| Failed | 503 | 503 | 启动或运行失败 |

新增 statez 输出状态、进入时间、Drain 原因和截止时间，不输出配置及凭据。

## 7. 服务迁移方案

### 7.1 bus 服务

标准启动依赖顺序：

1. 配置快照。
2. Logger、Tracing、Admin。
3. 数据和业务依赖。
4. SSRPC Registry Seal。
5. TransactionMgr。
6. Router/Bus。
7. 定时任务或 Gateway。
8. Ready。

标准关停顺序：

1. Ready=false。
2. Router 注销实例并拒绝新请求。
3. Gateway 或 Transport 停止新接入。
4. 等待会话和事务。
5. 刷新业务状态。
6. 关闭 Router/Bus。
7. 关闭数据依赖。
8. 关闭 Tracing、Admin、Logger。

### 7.2 web_svr

HTTP 和 gRPC 分别作为 Component：

- Start 必须确认 listener 建立成功。
- HTTP Drain 使用 Shutdown。
- gRPC Drain 使用 GracefulStop。
- 超时后 Stop 使用 Close/Stop。
- Reflection 由显式配置启用。
- Route 必须在 listener Start 前挂载完毕。

### 7.3 定时任务

删除 application 的 10ms Tick 和 Proc timer，改为受生命周期管理的 Task：

- mainsvr Role Tick：1 分钟。
- roomcentersvr Room Tick：5 秒。
- roomcentersvr Persist：10 秒。
- mysqlsvr ORM Monitor：30 秒。

Task 默认禁止重入。上一次未完成时跳过本次并记录指标，不创建叠加 goroutine。

## 8. 性能复审

### 8.1 当前基线

当前 Windows amd64 基准大致为：

| 项目 | 延迟 | 分配 |
|---|---:|---:|
| Transaction throughput | 634–680 ns/op | 441–446 B，7–8 allocs |
| Transaction serial key | 1587–1685 ns/op | 471–479 B，8 allocs |
| SSPacketHeader ToBytes | 3.52–3.59 ns/op | 0 alloc |
| SSPacketHeader To | 2.54–2.67 ns/op | 0 alloc |
| CSPacketHeader ToBytes | 12–16 ns/op | 32 B，1 alloc |
| bufpool Get/Put | 19.5–20 ns/op | 24 B，1 alloc |

不同机器结果不可直接比较。后续每个性能修改必须在同一机器、相同 Go 版本、相同参数下至少运行 10 次。

### 8.2 P0 可直接确认的性能问题

#### 无条件 GOMAXPROCS

删除。它既不是业务优化，也可能损害容器调度。

#### 全局 10ms Tick

大多数服务没有有效 Tick，却每秒被唤醒 100 次。roomcentersvr 还会在每次 Tick 创建两个 goroutine，即使管理器内部再自行节流。

替换为精确周期 Task 后：

- 空闲服务不再轮询。
- roomcenter 不再每秒创建约 200 个短命 goroutine。
- Task 可以被统一停止和观测。

#### Dispatcher RWMutex

注册只发生在启动期，运行期不应为每包查找支付读锁。Seal 后不可变 map 可以直接查找。

### 8.3 P1 内存优化

bufpool.Put 当前把局部 slice 地址放回 sync.Pool，稳态基准仍有一次分配。建议改为池化 Buffer Lease：

~~~go
type Buffer struct {
    Bytes []byte
}

func Acquire(size int) *Buffer
func Release(*Buffer)
~~~

Ownership 必须清楚：

- 入队成功后 writer 拥有 Lease。
- 入队失败由 sender 释放。
- writer 完成、出错或连接关闭都必须只释放一次。
- Stop 必须释放队列残留。

CSPacket 热路径应使用栈上 Header + To(dst)，避免先 ToBytes 再被 Gateway 复制。

### 8.4 暂不直接改 Transaction goroutine

当前每个在途 Transaction 启动 goroutine，但事务可能等待跨服务响应。简单改成每 shard 一个同步 worker 可能导致一个阻塞事务阻塞整个 shard。

只有满足以下证据之一才改变执行模型：

- goroutine 创建占 CPU profile 超过 10%。
- goroutine 栈内存成为容量上限。
- Transaction 分配占总分配超过 20%。
- 达到业务目标前已经出现 scheduler 瓶颈。

否则保留现有语义，只优化对象和 channel 分配。

### 8.5 gnet v2 与 writev 放入 P2

gnet v1 AsyncWrite 没有安全释放池化 buffer 的完成回调，因此当前合并复制是正确性优先的选择。

升级 gnet v2 和 AsyncWritev 必须独立实施，并满足：

- gnet copy 占 CPU 超过 10%，或
- 网关目标超过 5 万连接/实例，或
- write allocation 占总分配超过 15%。

没有 profile 证据时不把网络库升级和生命周期重构混在一起。

## 9. 简洁性与代码风格

### 9.1 应保留

- IDL-first SSRPC。
- Transaction serial key。
- 显式构造和小接口。
- logger 门面。
- 业务内部允许中文解释设计原因。
- 生成代码边界。

### 9.2 应删除

- application.Init/Run。
- ServiceApp Hook。
- OnProc/OnTick。
- RegisterToDispatcher/RegisterToTransactionMgr 双轨。
- Driver 注册型 init。
- driver/all 默认导入。
- package-global 可变配置。
- 启动路径中的 Fatal。
- 完整配置日志。

### 9.3 新增规范

- 每个 Runtime 资源必须由 Component 管理。
- 每个 goroutine 必须能由所属组件停止。
- 注册方法必须返回 error。
- 重复注册必须失败。
- 配置发布后不可修改。
- 热更新必须使用白名单。
- 性能修改必须附 benchmark。
- 异步 buffer 必须写明 ownership。
- Draining 必须先于 Stop。
- 新框架接口不使用 I 前缀。

## 10. 安全与运维风险

### 10.1 配置泄露

部分服务使用百分号加 v 打印完整配置，可能暴露：

- MQ 用户名密码。
- Redis/MySQL DSN。
- Sign secret。
- Tracing headers。

应改为只记录配置文件路径、服务名和已启用组件，敏感字段统一脱敏。

### 10.2 pprof 与反射

- admin IP 为空时不应默认暴露到所有网卡。
- pprof 只能挂载在 admin server。
- gRPC reflection 默认关闭，只在明确配置时启用。

### 10.3 Drain 不是会话迁移

本轮 Drain 只保证：

- 不接收新连接或新业务。
- 已有会话有等待窗口。
- 在途事务有机会完成。
- 超时后实例必然退出。

它不保证玩家无感迁移到另一实例。若未来需要无感迁移，需要额外设计重连票据、会话恢复和客户端协议。

## 11. 范围与默认决策

本轮已经确认：

- 采用核心现代化，不做最小补丁。
- 最终状态直接替换旧 API，不保留 deprecated adapter。
- 采用安全局部重载。
- Drain 默认 30 秒，Stop 默认再保留 10 秒，可按服务覆盖。
- 默认只链接 RabbitMQ Driver。
- Agones 只提供状态适配点，不直接集成。
- 不改变业务协议和持久化格式。
- 不在本轮升级 gnet v2、拆分多 Go module 或实现会话迁移。

## 12. 最终评价

GoOne 的竞争力不在于复制一个通用微服务框架，而在于把 Gateway、Bus、串行事务、Role、Room、配置数据和测试工具组合成一个适合游戏业务的运行时。

当前骨架方向正确，下一阶段最重要的不是继续增加更多抽象，而是完成以下闭环：

1. 每个资源有唯一生命周期所有者。
2. 每个注册错误在启动前暴露。
3. 每次发布都能停止接流、等待排空并受超时约束。
4. 每次配置更新都是原子、可解释的。
5. 每项性能优化都有基线、profile 和回归门禁。

具体任务、文件、顺序和验收命令见 [optimization_roadmap.md](optimization_roadmap.md)。
