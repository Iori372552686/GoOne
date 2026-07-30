# GoOne 架构评审 v3：现代化执行前基线

> 评审日期：2026-07-30  
> 评审范围：cmd/、src/、lib/、common/、module/、tools/、deploy/、.github/、docs/  
> 配套执行计划：[modernization_execution_plan_2026-07-v3.md](modernization_execution_plan_2026-07-v3.md)  
> 前置文档：[architecture_review_2026-07-v2.md](architecture_review_2026-07-v2.md)、[optimization_roadmap.md](optimization_roadmap.md)  
> 事实优先级：当前代码 > 配置与部署脚本 > CI > 文档；文档与代码冲突时以代码为准。

## 1. 目的

v2 评审与 [optimization_roadmap.md](optimization_roadmap.md) 的整改（旧编号体系，代码中以"方案 B"注释标记）已经落地了 `runtime.App + Component` 统一生命周期、生成 Registry 装配、显式 DriverRegistry、CI 静态门禁等基础。

本轮 v3 不再重复这些结论。本文档的唯一目的是：**在执行 v3 计划之前，逐项核对每个任务的真实完成度，冻结缺口证据，作为后续一切改造的可信基线。**

任务编号统一加 `V3-` 前缀（V3-P0-* / V3-P1-* / V3-BASE-* / V3-DEL-*），与旧 roadmap 和方案 B 的 P0/P1 编号解耦。状态采用四分类（已完成基线 / 兼容残留本轮删除 / 部分完成 / 待执行 / Deferred），不再沿用"待执行"一元标注。

## 2. 环境基线

| 项 | 值 |
|---|---|
| 分支 | `dev` |
| 提交 | `d00a5f452871df32993271b5e5cbf180375f3649` |
| Go（go.mod 指令） | `go 1.25.4` |
| Go（本地可用） | `go1.25.10 windows/amd64` |
| CGO_ENABLED | `0` |
| GOMAXPROCS | runtime 默认 |
| 采集机 | Windows 开发机（非正式基线机） |

> 正式性能基线、`govulncheck`、benchmark×10、容量矩阵 C1–C4 必须在固定 Linux 机器执行（见 v3 计划 PRE-02）。本机仅用于开发回归，结论不得直接作为生产容量结论。

## 3. CI 现状

`.github/workflows/ci.yml` 当前 3 个 job：

- `build-test`：build / vet / test / race（race 仅为该 job 内一个 step）。
- `check-genproto`：protoc 二次生成一致性。
- `lint`：golangci-lint（`version: latest`，未固定）+ 4 道静态门禁扫描 + 限定范围的文档链接检查。

已存在的静态门禁（均为前序迭代成果，v3 计划 P1-03/05 维护）：

1. legacy lifecycle API 扫描（`application.Init|Run`、`NewServiceApp`、`bootstrap.ServiceApp`）。
2. legacy SSRPC register API 扫描（`RegisterXxxToDispatcher`、`RegisterXxxToTransactionMgr`、`TransMgr.RegisterCmd`）。
3. `bus/driver/all` blank import 扫描（cmd/src/lib/service/runtime）。
4. 二进制依赖图门禁（websvr 不链 MQ SDK；connsvr 只链 rabbitmq；无 `streadway/amqp`）。

CI 缺口（v3 计划 P0-02 要求）：无独立 race job、**无 govulncheck**、无 middleware integration job、无容器、lint 版本未固定、文档链接检查未扩展到整个 docs 目录。

## 4. 任务真实完成度

> 状态分类（替代旧"待执行/部分完成"二元模型，避免与方案 B 旧 P0/P1 编号混淆）：
>
> - **已完成(基线)**：方案 B 已落地，CI 有防回归门禁，无需重做。
> - **兼容残留**：方案 B 过渡期兼容包装，生产主路径已不依赖，本轮删除退出。
> - **部分完成**：基座已存在，仅剩明确缺口，只列剩余差距。
> - **待执行**：V3 新增项，从零开始。
> - **Deferred**：条件性延后。
>
> 下表为勘察结论。V3 计划文档的任务总览表据此编号与状态（统一加 `V3-` 前缀）。

| V3 编号 | 任务 | 状态 | 核心缺口 / 证据 |
|---|---|---|---|
| V3-P0-01 | 安全/敏感信息 | 待执行 | 依赖升级全未做；Redis 密码、XORM DSN 明文入日志；reflection 无条件注册；无日志捕获测试 |
| V3-P0-02 | 测试分层/CI | 待执行 | `GOONE_INTEGRATION` 未落地；xorm 仍用 `os.Exit(0)`；CI 未拆 job、无 govulncheck |
| V3-P0-03 | Runtime 状态机 | 部分完成（~90%） | signal context 未前置到 Start 前；RuntimeError 未带组件名；3 个点名测试缺失 |
| V3-P0-04 | Gateway 回滚 | 部分完成（~50%） | **部分启动回滚完全未实现**（端口泄漏）；错误不含 tcp/ws/kcp 名；Stop 签名非 `Stop(ctx) error` |
| V3-P0-05 | 资源生命周期 | 部分完成 | Redis Manager 无 Close/无回滚；MySQL Worker 仍用 `init()` 启动；XORM 无 Close |
| V3-P0-06 | Web 生命周期 | 部分完成 | HTTP Server 无任何超时配置；Quiesce 不翻 health；reflection 无开关；资源未拆组件 |
| V3-P0-07 | 风格/文档 | 待执行 | 无 `.gitattributes`；170 处历史注释未清理；protoc 插件未接 go/format；docs 12 处断链 |
| V3-P1-01 | 过载保护 | 待执行 | capacity 配置字段全缺；无连接上限/登录限速/admission mw/ErrOverloaded/rejected 指标 |
| V3-P1-02 | SessionHub 单路径 | 部分完成 | 共享 Hub 已落地；但本地 map/锁未删、`hub!=nil` 双分支约 23 处仍在、严格 BusID/IP 解析接口缺失 |
| V3-P1-04 | 配置不可变 | 部分完成（~40%） | `appconfig.Store` 设计完备但生产未采用；gconf 仍 6 个可变全局；业务层直读全局；无 Deprecated 标记 |
| V3-P1-05 | Driver 契约/重连 | 部分完成 | DriverRegistry 已达成；driver 层无 Start/Quiesce/Drain/Stop 契约；无断线重连测试 |
| V3-P1-06 | 容量工具 | 部分完成（~35%） | 有 stress 工具但缺序列号关联、显式阶段机、多机分片、JSON 原始数据、容量矩阵、多数资源指标 |
| V3-P1-07 | 性能优化 | 部分完成（~65%） | CS/SS Header.To 0-alloc 已落地、TCP/KCP 热路径已改造；WS 热路径仍 1-alloc；TransactionMgr 有 bench/profile 门禁文档 |

## 4a. 方案 B 已验证基线（无需重做）

以下为方案 B 已落地且 CI 有防回归门禁的能力。v3 计划不再重复实施，仅在本轮删除其过渡期兼容残留（见 V3-DEL-02）。

| 能力 | 状态 | 证据 |
|---|---|---|
| 旧生命周期删除（application/bootstrap） | 已完成(基线) | 六服务 `NewApp().Run(ctx)`；CI legacy lifecycle 扫描（`ci.yml:99-114`） |
| 服务装配（RegistryComponent） | 已完成(基线) | 五 Bus 服务 `NewRegistryComponent`+`RegisterToRegistry`；Web `Registry→Seal`；CI legacy SSRPC register 扫描（`ci.yml:116-132`） |
| 显式 DriverRegistry 装配 | 已完成(基线) | 五 Bus 服务显式构造 `bus.NewDriverRegistry()` 只注册 RabbitMQ（如 `src/mainsvr/app.go:82-83`） |
| 二进制依赖图裁剪 | 已完成(基线) | websvr 不含 MQ SDK；connsvr 只链 amqp091-go；CI `go list -deps` 门禁（`ci.yml:151-165`），实测通过 |
| pprof 双开关 | 已完成(基线) | `lib/service/runtime/admin.go`（enabled+pprof+回环），五服务接线一致 |
| Tracing 不记敏感信息 | 已完成(基线) | trace.go 只记 span 名/cost |
| CSPacketHeader/SSPacketHeader 0-alloc `To` | 已完成(基线) | `lib/api/sharedstruct`；TCP/KCP 网关热路径已用 `To`+栈数组 |

兼容残留（生产主路径已不依赖，本轮删除退出）：

| 残留 | 证据 | 处置 |
|---|---|---|
| `bus/driver/all` 兼容包 | `lib/service/bus/driver/all/all.go`；CI 已禁 cmd/src/lib/service/runtime 导入 | 已删除（V3-DEL-02） |
| 包级 Driver `init()` 注册 | 各 driver 的 `func init()`；`RegisterBus`/`CreateBus`（`bus_factory.go`） | 已删除（V3-DEL-02）；`BusCtor` 类型迁移至 driver_registry.go |
| 旧 SSRPC 注册包装 | `RegisterXxxToDispatcher` 等；生产代码无调用（CI 门禁证实），仅生成代码/兼容实现保留 | 生成包装本轮保留（属生成代码，改生成器留独立任务）；手写兼容层已删 |
| 生产代码历史注释 | 170 处"方案B/P0-xx/P1-xx/roadmap"（43 文件）+ ci.yml 10 行 | 本轮按字面删除（V3-P0-07） |

## 5. 各任务缺口证据（file:line）

### V3-P0-01 安全漏洞与敏感信息治理

| 子项 | 状态 | 证据 |
|---|---|---|
| Go 工具链 ≥1.25.12 | 未完成 | `go.mod:3` = `1.25.4`；本地 = 1.25.10 |
| gRPC ≥1.82.1 | 未完成 | `go.mod` = `v1.77.0` |
| OpenTelemetry ≥1.43.0 | 未完成 | `go.mod` = `v1.38.0` |
| x/net ≥0.55.0 | 未完成 | `go.mod` = `v0.47.0` |
| x/text ≥0.39.0 | 未完成 | `go.mod` = `v0.31.0` |
| quic-go ≥0.59.1 | 未完成 | `go.mod` = `v0.54.0` |
| Redis 日志脱敏 | **未完成** | `lib/db/redis/redis_mgr.go:43` 打印含 `Password` 字段的 `Config`（`config.go:8`） |
| XORM 脱敏 DSN | **未完成** | `lib/db/xorm/orm_engin.go:74` 打印含 `user:password@...` 的完整 DSN |
| Tracing 不记 Header/Token | 已完成 | trace.go 只记 span 名/cost |
| gRPC reflection 仅 debug | **未完成** | `src/web_svr/app.go:146` 无条件 `reflection.Register(srv)` |
| pprof 双开关 | 已完成 | `lib/service/runtime/admin.go`（enabled + pprof + 回环），五服务接线一致 |
| 日志捕获测试 | 未完成 | 全仓无相关断言 |
| 潜在旁路 | 风险 | `lib/db/ssdb/test.go` 含 `func main()` 且 blank-import `_ "net/http/pprof"`，会注册到 `http.DefaultServeMux`，绕过 admin 双开关 |

### V3-P0-02 测试分层与 CI 可信性

| 子项 | 状态 | 证据 |
|---|---|---|
| 统一 `GOONE_INTEGRATION=1` | 未完成 | 代码零引用，仅文档存在 |
| xorm 不用 `os.Exit(0)` | 未完成 | `lib/db/xorm/xorm_test.go:22,27,32` 仍 `os.Exit(0)` |
| 各测试门控 | 不统一 | bus=`BUS_ITEST=1`；redis cap_test 无门控直连 `10.0.0.173:6379` 硬编码；nacos 预检 2s（超 500ms-1s）；nsq cap_test 永久 `t.Skip` |
| CI job 拆分 | 未完成 | 仅 build-test + check-genproto + lint；race 非独立 job |
| govulncheck | 未完成 | 全仓库无调用 |
| integration job + 容器 | 未完成 | 无 `services:`/docker-compose |
| 固定 lint/vulncheck 版本 | 未完成 | `version: latest` |
| 文档链接检查扩全 docs | 未完成 | 仅扫 readme/CHANGELOG 引用的 docs/*.md |

### V3-P0-03 Runtime 状态机

已完成项：状态机（`lib/service/runtime/state.go`，含 `StateFailed`）、`markFailed` 触发（`run.go:365`）、统一 reason 汇聚首条即进入 Draining（`run.go:192-214`）、Drain `context.WithCancelCause`+`WithTimeout`（`run.go:255-266`）、组件级逆序 Stop 回滚（`run.go:109-140`）、observer 回调读状态测试。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| signal context 前置到 Start 前 | 未完成 | `installSignals()`（`run.go:54`）在 `startComponents`（`run.go:45`）之后 |
| 用 `signal.NotifyContext` | 未完成 | 用手写 dispatcher（`signal.go:44-109`），功能等价但不符字面 |
| RuntimeError 带组件名 | 未完成 | `run.go:164-179` 透传原始 error，无 `fmt.Errorf("component %q: %w", ...)` |
| transition 锁覆盖 observer | 偏差 | observer 在 `mu.Unlock()` 后执行（`state.go:176-190`），与注释"持锁派发"不符 |
| SIGTERM 到达慢 Start 测试 | 缺失 | — |
| Allocate 与 Drain 并发测试 | 缺失 | — |

### V3-P0-04 Gateway 部分启动回滚

已完成项：传输级 Stop 幂等（`sync.Once`：`tcp_server.go:68/80`、`ws.go:86/98`、`kcp.go:73/85`）；Quiesce 关 listener 拒新会话（`app.go:161-168`）；Drain 等 ActiveSessions（`app.go:172-174`）；Stop 强制处理剩余连接。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| **部分启动回滚** | **未实现** | `gatewayComponent.Start`（`src/connsvr/app.go:141-157`）顺序启动无已启动列表、无逆序 Stop，WS/KCP 失败时 TCP 端口泄漏 |
| 错误含 tcp/ws/kcp 名 | 未实现 | `net_factory.go` 与 `app.go:178-184` 直接 `return err` |
| Stop 签名 `Stop(ctx) error` | 不符 | 实际 `Stop()` 无参无返回；`gatewayComponent.Stop` 忽略 ctx 返回 nil |
| 网关回滚测试 | 缺失 | net_mgr/tcp/ws 测试无相关用例 |

### V3-P0-05 资源生命周期

| 子项 | 状态 | 证据 |
|---|---|---|
| Redis `Close(ctx) error` | 未完成 | `lib/db/redis/redis_mgr.go` 无 Close |
| Redis 多实例失败回滚 | 未完成 | `InitAndRun`（`redis_mgr.go:42-54`）失败直接 return，不关已成功实例 |
| XORM `OrmManager.Close` | 未完成 | `lib/db/xorm/orm_mgr.go` 无 Close |
| XORM SyncTables 失败关 Engine | 未完成 | `orm_engin.go:90-92` 仅 return err |
| mysqlsvr Stop 关 ORM Engine | 未完成 | `src/mysqlsvr/app.go:46-50` 只关 async workers |
| MySQL Worker 移除 init() | **未完成** | `src/mysqlsvr/manager/table_mgr.go:25-37` 仍 `init()` 启动 worker + 注册表 |
| Worker Component 化 | 未完成 | `lib/service/async/async.go` 仅 Start/Push/Stop，无 Quiesce/Drain |
| Nacos config 错误返回 | 已完成 | `lib/contrib/config/nacos/nacos.go:56` 返回 error |
| Nacos registry 错误返回 | 未完成 | `lib/contrib/registry/nacos/registry.go:64` 返回单值 `*Registry` |
| Nacos ListenConfig 记录/取消 | 已完成 | `watcher.go:23-69` |
| Gamedata 不可变快照/原子替换 | 部分 | gen 层 `atomic.Value` 满足；中心 `gamedata.go:55-64` 无版本通知/校验封装，OnChange 失败仅记日志 |

### V3-P0-06 Web HTTP/gRPC 生命周期

已完成项：HTTP/gRPC server 实例化（`src/web_svr/app.go:37-44`）；gRPC Listen-then-Serve（`app.go:134`）；RuntimeErrors channel（`app.go:50-52`）；Stop 超时强制关闭设计（`app.go:108-122,174,187` 保留指针供强关）。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| HTTP Listen-before-Serve | 未完成 | `lib/web/web_gin/http.go:30-42` 直接 `go ListenAndServe()` |
| HTTP 超时配置 | **未完成** | `web_gin/config.go` 无字段，`http.go:69-72` 构造 Server 无 Timeout/MaxHeaderBytes |
| Quiesce 翻 health NOT_SERVING | 未完成 | `app.go:102-104` 只 Shutdown；全库无 `NOT_SERVING` |
| HTTP Serve 错误进 channel | 未完成 | 仅 gRPC 路径上报（`app.go:87-88` 注释自述） |
| reflection debug 开关 | 未完成 | `app.go:146` 无条件 Register |
| 资源拆为独立组件 | 未完成 | `app.go:66-98` Redis/签名/敏感词/HTTP/gRPC 同一组件 |

### V3-P0-07 代码风格和文档一致性

| 子项 | 状态 | 证据 |
|---|---|---|
| `.gitattributes` | **未存在** | 仓库根无该文件；ci.yml 仍 CRLF |
| 清理历史注释 | 未开始 | 170 行/43 文件（方案B 4 处、roadmap 18 处、P0/P1/P2 约 148 处）；ci.yml 另 10 行 |
| protoc 插件接 go/format | 未完成 | `tools/protoc-gen-goone/generate.go` 不 import go/format，Buffer 直接拼接 |
| docs 内部链接有效 | 部分失效 | 12 处断链，如 `architecture_review_2026-07-v3.md`（被引用不存在）、`capacity-matrix.md`、`modernization-b-baseline.md` |

### V3-P1-01 Admission Control

| 子项 | 状态 | 证据 |
|---|---|---|
| capacity 配置字段 | 未完成 | `module/gconf/config.go:112-119` ConnRuntimeConfig 仅 ListenPort/TcpImplType/KcpPort；`ConnSvr`（`config.go:137-140`）无 Capacity |
| 连接数检查/未认证上限 | 未完成 | `tcp_server.go:160-181` Accept 后直接 OnConn |
| 登录限速（全局+IP） | 未完成 | `src/connsvr/login/login.go` 无限速 |
| SSRPC admission mw | 未完成 | `lib/service/ssrpc/default_mw.go:28-43` 无 admission |
| ErrOverloaded | 未完成 | `errors.go:14-33` 无定义 |
| rejected/queue/wait 指标 | 未完成 | 仅有 inflight/active-conn 基础（可复用） |

### V3-P1-02 SessionHub 单路径化

已完成项：共享 `SessionHub` 设计与生产注入（`lib/net/net_mgr/session_hub.go`、`src/connsvr/globals/globals.go:20-45` 三传输共享同一 Hub）。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| 删除本地 map/锁 | 未完成 | `net_i.go:49-97` 三结构体仍持有本地 map+RWMutex |
| 删除 `hub!=nil` 双分支 | 未完成 | 约 23 处：tcp_impl.go（9）、ws_impl.go（5）、kcp_impl.go（9） |
| SetHub 标 Deprecated | 未完成 | `net_i.go:117-121` 无 Deprecated |
| 公开构造函数 hub==nil | 风险 | `NewTcpSvr` 等产出 hub==nil 实例走本地 map |
| 严格 BusID/IP 解析 | 未完成 | `lib/service/bus/bus_ip.go:9-54` 全无 error 返回；无"仅 IPv4"声明 |

### 服务装配 / 显式 Driver / 二进制裁剪 — 已完成(基线)

归入 §4a 方案 B 已验证基线（V3-BASE-01/02/03、V3-DEL-01），无需重做。本轮仅删除其过渡期兼容残留（V3-DEL-02）。

### V3-P1-04 配置不可变与全局状态收敛

| 子项 | 状态 | 证据 |
|---|---|---|
| `appconfig.Store` 设计 | 已完成 | `lib/service/appconfig/store.go:53-159` 不可变/原子/白名单 Reload，有测试 |
| 生产采用 Store | 未完成 | cmd/src/module/lib 生产代码 0 处使用 |
| gconf 不可变 | 未完成 | `module/gconf/config.go:177-217` 仍 6 个可变包级全局 |
| 业务层收敛 | 未完成 | `sync_state.go:293,345,348`、`base.go:56`、`bindings.go:19` 等直读全局 |
| Deprecated 标记 | 未完成 | Store/WithReload/Module Registry 全仓 0 处 Deprecated |

### V3-P1-05 显式 Driver 与依赖裁剪

已完成项：`bus.DriverRegistry` + 各服务 `drivers.MustRegister(rabbitmq.Driver())`；websvr 不含 MQ；CI 二进制裁剪门禁。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| 包级注册/init() 标 Deprecated | 未完成 | `bus_factory.go:17,26`、各 driver `init()` 未标 |
| driver 生命周期契约 | 未完成 | `IBus`（`bus_i.go:22-35`）仅 Send/Close/Healthy；Quiesce/Drain/Stop 在上层 RouterComponent |
| 断线重连测试 | 未完成 | 重连逻辑在 `rabbitmq.go:162-191`，但无自动化测试 |

### V3-P1-06 容量工具升级

已有：stress 工具复用真实协议、ramp-up、重连、Markdown 报告、服务端 CPU/heap/goroutine 采集。

缺口：序列号关联（现按 cmd+1 FIFO）、显式阶段机（steady/drain 独立）、多进程多机分片、JSON 原始数据、容量矩阵 Markdown、客户端资源采集、服务端 RSS/GC/FD/Session、P999。

### V3-P1-07 基于证据的性能优化

已完成项：`CSPacketHeader.To(b)`、`SSPacketHeader.To` 0-alloc 写已有缓冲区；TCP/KCP 网关热路径已用 `To`+栈数组；`ToBytes` 保留为兼容接口；TransactionMgr bench 基线（`docs/benchmarks/baseline.md`）+ profile 证据门禁文档。

缺口：

| 子项 | 状态 | 证据 |
|---|---|---|
| WS 网关下行热路径 | 未完成 | `lib/net/net_mgr/ws_impl.go:241` 仍用 `ToBytes()`（1-alloc） |
| CS Header.ToBytes | 非 0-alloc | 1-alloc（32B），保留为兼容接口，非发送热路径 |

## 6. 与主流框架能力差异（保持有效）

| 框架 | GoOne 现状 | 本轮目标 |
|---|---|---|
| Kratos | 已有统一 Component Start/Stop | 补齐部分启动回滚与 Failed 终态语义 |
| Pitaya | 已有 Quiesce/Drain 接口 | 落实网关传输级回滚与 HTTP/gRPC Drain |
| due | 已有显式 DriverRegistry | driver 生命周期契约与断线重连测试 |
| Agones | 保留 VM/Ansible + 边界 | 不提前引入 SDK（V3-P2-03 维持 Deferred） |

## 7. 待执行优先级

依据 §4 完成度与依赖，本轮执行顺序：

1. V3-DEL-02 删除兼容残留（净化基线，让后续工作基于唯一主路径）。
2. V3-P0-01 安全治理：依赖升级与脱敏（安全阻断项优先）。
3. V3-P0-02 测试分层与 CI 拆分（为后续所有验收提供可信基础）。
4. V3-P0-03 / 04 / 05 / 06 生命周期补齐。
5. V3-P0-07 风格与文档（本轮触及文件清理，单独提交）。
6. V3-P1 系列按依赖推进。

## 8. 待 Linux 验证项（本机无法完成）

以下项必须由固定 Linux 机器执行并回填：

- PRE-02 正式基线：build/vet/test/race/govulncheck 完整结果。
- benchmark×10（`lib/api/sharedstruct`、`lib/service/transaction`、`lib/service/ssrpc`、`lib/net`）+ benchstat 对比。
- 容量矩阵 C1–C4（单 connsvr 10,000 长连接 SLO）。
- 二进制依赖图门禁（`go list -deps ./cmd/*`）的 Linux 确认。
