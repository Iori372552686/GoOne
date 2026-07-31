# GoOne 二次现代化详细执行方案

> **⚠️ 已被 v4 取代（2026-07-31 核销）**：本文档的 V3-P0-* 任务已全部完成并验证，
> 后续迭代由 [`modernization_execution_plan_2026-07-v4.md`](modernization_execution_plan_2026-07-v4.md)
> 接管。本文保留作为 V3 阶段的历史决策与验收证据归档；其中任何「待执行 / 执行中」
> 描述均已过时，**当前事实以 v4 计划与代码为准**（事实优先级：代码 > v4 计划 > 本文档）。
>
> V3 任务状态核销：V3-P0-*（生命周期串行化、admission、gateway lease、资源 Component、
> gamedata、router bus、httpclient、CI 分层）= 已完成并进入 v4 基线；V3-P1-* 容量与
> 可观测性项由 v4 的 P1-03~P1-05 重新定义并推进。

> 文档状态：执行中（2026-07-30 完成 PRE-01 现状审计）  
> 编制日期：2026-07-30  
> 适用范围：GoOne 全仓库  
> 前置文档：[架构审查 v2](architecture_review_2026-07-v2.md)、[架构评审 v3（执行前基线）](architecture_review_2026-07-v3.md)、[现有优化路线图](optimization_roadmap.md)、[代码风格规范](STYLE.md)  
> 说明：本文定义执行方案、优先级和验收门禁。"状态"列反映截至 2026-07-30 的真实代码完成度勘察，详见 v3 评审文档 §4-5。

## 1. 目标与优先级

本轮目标不是继续增加框架概念，而是把已经完成的方案 B 收敛成可验证、可运维、可承载正式业务的生产框架。

| 等级 | 定义 | 合并条件 |
|---|---|---|
| P0 | 正确性、安全、资源泄漏、CI 可信性问题 | 全部完成后才能发布下一稳定版本 |
| P1 | 过载保护、容量证明、单路径化、配置与注册简化 | 全部完成后才能声明“生产级容量闭环” |
| P2 | 依赖代际升级、Agones、gnet v2、PGO、破坏性清理 | 必须满足量化收益或明确部署需求后实施 |

总体完成标准：

- 默认测试不依赖开发机中间件。
- `govulncheck` 无可达漏洞。
- 所有长期资源均由唯一 Component 持有并关闭。
- 生命周期不存在并发状态覆盖和遗留 goroutine。
- 单实例 connsvr 达到 10,000 长连接验收目标。
- 生产代码不再维护两套注册、Session、配置读取主路径。
- 代码、文档、CI、基准数据描述一致。

## 2. 执行任务总览

> 状态分类（替代旧"待执行/部分完成"二元模型，避免与方案 B 旧编号混淆）：
>
> - **已完成(基线)**：方案 B 已落地，CI 有防回归门禁，无需重做。
> - **兼容残留(本轮删除)**：方案 B 过渡期的兼容包装，生产主路径已不依赖，本轮直接删除。
> - **部分完成**：基座已存在，仅剩明确缺口，只列剩余差距。
> - **待执行**：V3 新增项，从零开始。
> - **Deferred**：条件性延后，满足量化门槛或明确需求后才实施。
>
> 任务编号统一加 `V3-` 前缀，与旧 roadmap 的 P0/P1 编号解耦。

| 编号 | 任务 | 来源 | 状态 | 剩余差距 / 证据 |
|---|---|---|---|---|
| PRE-01 | 冻结执行文档和审计证据 | V3 | 已完成 | v3 评审已建，缺口证据见 architecture_review_2026-07-v3.md §4-5 |
| PRE-02 | 冻结构建、测试、安全和性能基线 | V3 | 待 Linux 验证 | 本机为 Windows 开发机；正式基线须固定 Linux 机器执行 |
| V3-P0-01 | 安全漏洞与敏感信息治理 | V3 新增 | 代码侧完成，待 Linux 验证 | 依赖升级4组已完成(gRPC1.82.1/OTel1.43/x-net0.56/x-text0.39/quic0.59.1)；脱敏/reflection开关/pprof旁路已完成；race/govulncheck待Linux |
| V3-P0-02 | 测试分层与 CI 可信性 | V3 新增 | 待执行 | `GOONE_INTEGRATION` 未落地；xorm 仍 `os.Exit(0)`；CI 未拆 job、无 govulncheck |
| V3-P0-03 | Runtime 状态机与启动期信号 | 方案B基础+V3补齐 | 部分完成（~90%） | 缺：signal context 前置到 Start 前；RuntimeError 带组件名；3 个点名测试 |
| V3-P0-04 | Gateway 部分启动回滚 | 方案B基础+V3补齐 | 部分完成（~50%） | 缺：传输级回滚（端口泄漏）；错误含 tcp/ws/kcp 名；`Stop(ctx) error` 签名 |
| V3-P0-05 | Redis、XORM、Nacos、Gamedata 生命周期 | 方案B基础+V3补齐 | 部分完成 | Nacos config 已完成；缺：Redis/XORM Close 与失败回滚；MySQL Worker 移除 init() 并 Component 化 |
| V3-P0-06 | Web HTTP/gRPC 生命周期 | 方案B基础+V3补齐 | 部分完成 | gRPC Listen-first+Stop 强关已完成；缺：HTTP 超时配置；Quiesce 翻 health；reflection 开关；资源拆组件 |
| V3-P0-07 | 代码风格和文档一致性 | V3 新增 | 待执行 | 无 `.gitattributes`；170 处历史注释；protoc 未接 go/format；docs 断链 |
| V3-P1-01 | Admission Control 与过载保护 | V3 新增 | 待执行 | capacity 配置字段全缺；无连接上限/登录限速/admission mw/ErrOverloaded/rejected 指标 |
| V3-P1-02 | SessionHub 单路径化 | 方案B基础+V3补齐 | 部分完成 | 共享 Hub 已落地；缺：本地 map/锁、`hub!=nil` 双分支（~23 处）；严格 BusID/IP 解析；SetHub 退出 |
| V3-BASE-01 | 服务装配简化（RegistryComponent） | 方案 B | **已完成(基线)** | CI 门禁真实合规；六服务经 RegistryComponent 装配 |
| V3-BASE-02 | 显式 DriverRegistry 装配 | 方案 B | **已完成(基线)** | 五 Bus 服务显式构造 DriverRegistry 只注册 RabbitMQ |
| V3-BASE-03 | 二进制依赖图裁剪 | 方案 B | **已完成(基线)** | websvr 无 MQ SDK；connsvr 只链 amqp091-go |
| V3-DEL-01 | 旧生命周期删除（application/bootstrap） | 方案 B | **已完成(基线)** | 六服务 NewApp().Run；CI 防回归扫描 |
| V3-DEL-02 | 删除兼容残留：driver/all、包级 Driver `init()`、`RegisterBus`/`CreateBus`、旧注册包装 | 方案B过渡期 | **兼容残留(本轮删除)** | 生产主路径已不依赖；CI 已禁 driver/all 生产导入；本轮直接删除退出 |
| V3-P1-04 | 配置不可变与全局状态收敛 | V3 新增 | 部分完成（~40%） | appconfig.Store 设计完备但生产未采用；gconf 仍可变全局；业务层直读全局 |
| V3-P1-05 | Driver 生命周期契约与断线重连测试 | 方案B基础+V3补齐 | 部分完成 | DriverRegistry 已达成；缺：driver 层 Start/Quiesce/Drain/Stop 契约；断线重连自动化测试 |
| V3-P1-06 | 容量工具升级 | V3 新增 | 部分完成（~35%） | stress 工具基础在；缺：序列号关联/显式阶段机/多机分片/JSON/矩阵/多数资源指标 |
| V3-P1-07 | 基于证据的性能优化 | V3 新增 | 部分完成（~65%） | CS/SS Header.To 0-alloc+TCP/KCP 已改造；缺：WS 热路径仍 1-alloc |
| V3-P2-01 | gnet v2 A/B 验证 | V3 | Deferred | 仅 Linux A/B 达门槛（吞吐+10% 或 CPU/RSS-15%）才实施 |
| V3-P2-02 | PGO 验证 | V3 | Deferred | 容量矩阵稳定提升≥5% 才实施 |
| V3-P2-03 | Agones Adapter | V3 | Deferred | 明确 Agones 部署需求后才实施 |
| V3-P2-04 | 下一主版本破坏性清理 | V3 | Deferred | 一个弃用周期完成后实施 |
| V3-P2-05 | 基础依赖现代化 | V3 | Deferred | P0/P1 稳定后实施 |

执行顺序（依据状态与依赖）：先 V3-DEL-02（删兼容残留，净化基线）→ V3-P0-01 → V3-P0-02 → V3-P0-03/04/05/06 → V3-P0-07 → V3-P1 系列。

## 3. 执行前准备

### PRE-01 冻结执行文档

维护以下文档：

- `docs/architecture_review_2026-07-v3.md`（执行前基线 + 缺口证据）
- `docs/modernization_execution_plan_2026-07-v3.md`（本文件，任务状态与进度）

架构审计文档必须记录：

- 当前 commit、分支、Go 版本、操作系统。
- build、vet、test、race、govulncheck 结果。
- 当前微基准数据。
- 当前失败项及复现命令。
- 与 Kratos、Pitaya、due、Agones 的能力差异。
- 每个结论对应的代码证据。

执行计划文档必须持续记录：

- 任务状态和负责人。
- 开始、完成日期。
- 验收命令和结果。
- 未完成项和阻塞原因。
- 回滚方式。

验收标准：

- `docs/**/*.md` 内部链接全部有效。
- 不再引用不存在的基准或容量文档。
- 未实际完成的任务不得标记为“完成”。

### PRE-02 冻结当前基线

执行并保存以下结果：

```bash
go version
go env GOOS GOARCH CGO_ENABLED
git rev-parse HEAD
go build ./...
go vet -composites=false ./...
go test -count=1 -timeout 600s ./lib/... ./src/... ./common/... ./module/... ./tools/protoc-gen-goone/... ./tools/cmd/...
govulncheck ./...
```

微基准至少执行 10 次：

```bash
go test -run '^$' -bench . -benchmem -count=10 \
  ./lib/api/sharedstruct/... \
  ./lib/service/transaction/... \
  ./lib/service/ssrpc/... \
  ./lib/net/...
```

执行注意点：

- 当前数据只作为改造前基线，不能直接作为生产容量结论。
- Windows 数据只做开发机回归，正式性能结论必须来自固定 Linux 机器。
- benchmark 期间关闭 Info 日志。
- 原始输出保存到忽略目录，摘要和环境信息写入文档。
- 不同机器、Go 版本或 GOMAXPROCS 的数据不得直接比较。

## 4. P0：发布阻断项

P0 未全部通过前，不开始性能结构调整，也不能发布下一稳定版本。

### V3-P0-01 安全漏洞与敏感信息治理

#### 当前问题

当前安全扫描存在可达漏洞，涉及：

- Go 标准库。
- gRPC。
- OpenTelemetry。
- `x/net`。
- `x/text`。
- `quic-go`。

同时存在 Redis、XORM/MySQL 日志输出完整连接配置或 DSN 的风险。

#### 执行步骤

1. 工具链升级到 Go 1.25.12 或同系列更高安全补丁。
2. 按依赖族分组升级，每组独立提交：
   - gRPC ≥ 1.82.1。
   - OpenTelemetry API、SDK、Exporter 统一到 ≥ 1.43.0。
   - `x/net` ≥ 0.55.0。
   - `x/text` ≥ 0.39.0。
   - `quic-go` ≥ 0.59.1。
3. 每组升级后执行 build、test、race 和协议兼容测试。
4. Redis 日志只允许记录实例 ID、地址、DB 和连接池大小。
5. XORM/MySQL 使用脱敏 DSN，错误对象不得携带明文密码。
6. Tracing Header、Token、完整配置内容不得输出到普通日志。
7. gRPC reflection 仅在明确 debug 配置下启用。
8. pprof 必须同时满足 admin enabled 和 pprof enabled。

#### 验收方法

```bash
go build ./...
go vet -composites=false ./...
go test -count=1 ./...
govulncheck ./...
```

增加日志捕获测试，输入包含已知密码、Token、DSN，断言输出中不存在这些内容。

#### 验收标准

- `govulncheck ./...` 可达漏洞为 0。
- 日志和错误中不存在密码、Token、完整 DSN。
- `go mod tidy` 后没有非必要依赖漂移。
- 六个服务全部可构建。
- gRPC、HTTP、SSRPC 回归测试通过。

#### 执行注意点

- 不一次性升级所有依赖到 latest。
- 不使用 `replace`、忽略规则或缩小扫描范围掩盖漏洞。
- OTel 相关模块保持相同版本系列。
- 依赖升级提交不得混入生命周期重构。
- 每组依赖升级都必须有独立回滚提交点。

### V3-P0-02 测试分层与 CI 可信性

#### 当前问题

默认全量测试会隐式连接本地 etcd，并在环境缺失时超时失败，不符合 [代码风格规范](STYLE.md) 中“预检后 `t.Skip`”的要求。

#### 执行步骤

1. 将中间件测试划分为：
   - 单元测试：fake、stub 或内存实现。
   - 集成测试：真实 etcd、Redis、MySQL、RabbitMQ、Nacos。
2. 集成测试统一检查 `GOONE_INTEGRATION=1`。
3. 未开启集成模式时使用 `t.Skip`。
4. 开启集成模式后，先执行 500ms～1s 的端口或 Ping 预检。
5. CI 拆分为：
   - unit/build job。
   - race job。
   - middleware integration job。
   - generated-code job。
   - vulnerability/lint job。
6. integration job 启动真实服务容器。
7. integration job 必须确认真实执行的用例数大于 0。
8. 固定 golangci-lint 和 govulncheck 版本。
9. 文档链接检查扩展到整个 `docs` 目录。
10. xorm 测试删除使用 `os.Exit(0)` 跳过整个 package 的做法。

#### 验收方法

无中间件环境：

```bash
go test -count=1 ./...
```

集成环境：

```bash
GOONE_INTEGRATION=1 go test -count=1 -tags=integration ./lib/contrib/... ./lib/db/...
```

#### 验收标准

- 无中间件时默认测试通过或明确 Skip，不等待网络超时。
- 集成环境中真实中间件用例确实执行。
- CI 能区分测试成功、环境缺失 Skip、环境存在但测试失败。
- 测试结束后无残留连接、listener 和 goroutine。

#### 执行注意点

- 不通过缩短 deadline 把依赖缺失伪装成普通测试失败。
- 不允许 integration job 全部 Skip 后仍显示成功。
- 外部依赖测试必须通过 `t.Cleanup` 释放资源。
- 单元测试不得访问固定开发地址。

### V3-P0-03 Runtime 状态机与启动期信号

#### 当前问题

- 信号监听在组件全部启动后才安装。
- Allocate observer 与 Drain 可能并发覆盖状态。
- 运行时错误缺少组件身份。
- 一个退出原因先到达后，其他等待 goroutine 不能立即结束。
- Drain/Stop 失败后的终态表达不准确。

#### 执行步骤

1. `Run` 入口立即创建 signal context。
2. LoadConfig 和所有 Component Start 使用该 context。
3. 启动中收到 SIGTERM/SIGINT 时：
   - 当前 Start 收到取消。
   - 已成功启动组件逆序 Stop。
   - App 不进入 Ready。
4. 增加迁移串行锁，覆盖状态校验、observer 调用和状态提交。
5. 状态读取继续使用读锁，不被 observer 阻塞。
6. 明确 observer 不允许递归发起状态迁移。
7. 任一退出原因出现后立即取消全部等待者。
8. 运行时错误包装组件名并保留 `Unwrap`。
9. Drain 或 Stop 失败时终态设置为 Failed。
10. Tracker 保存最后迁移原因，删除重复的 drainReason 状态来源。
11. 修复 observer 锁说明、Admin enable 行为和重复注释。

#### 必须增加的测试

- SIGTERM 到达慢 Start。
- 第 N 个组件启动失败。
- Allocate 与 Drain 并发。
- RuntimeError 与 SIGTERM 同时到达。
- 父 context cancel 与组件错误同时到达。
- Drain 超时。
- Stop 超时。
- Stop 返回多个错误。
- 重复 Allocate、Drain、Stop。
- observer 在回调期间读取当前状态。

#### 验收方法

```bash
go test -count=100 ./lib/service/runtime/...
go test -race -count=20 ./lib/service/runtime/...
```

#### 验收标准

- 无数据竞争和状态回退。
- 每个已启动组件只 Stop 一次。
- 停止顺序严格为启动顺序的逆序。
- Run 返回后没有遗留 watcher。
- 错误中包含组件名、阶段和根因。

#### 执行注意点

- 不把状态机扩展成事件总线。
- 不在持有 state mutex 时执行可能阻塞的外部调用。
- transition mutex 可覆盖 observer，但 observer 不得重入状态迁移。
- `errors.Join` 后仍需保留组件名和生命周期阶段。

### V3-P0-04 Gateway 部分启动回滚

#### 当前问题

Gateway 按 TCP、WS、KCP 顺序启动。后一个启动失败时，前面已启动的监听器不会自动关闭，而 App 不会对 Start 失败的当前组件调用 Stop。

#### 执行步骤

1. Gateway 记录已启动传输列表。
2. 任一 Start 失败时：
   - 立即逆序停止已启动传输。
   - 合并原始启动错误与回滚错误。
   - 返回包含传输名称的错误。
3. TCP、WS、KCP Stop 接收 context 并返回 error。
4. 所有 Stop 保证幂等。
5. gnet Serve 的运行期错误进入 RuntimeErrorSource。
6. gnet Stop 使用调用方 context。
7. Quiesce 只关闭 listener 和拒绝新 Session。
8. Drain 等待已认证 Session。
9. Stop 强制处理剩余连接。

#### 验收场景

- TCP 端口占用。
- WS 端口占用但 TCP 已启动。
- KCP 端口占用但 TCP、WS 已启动。
- Quiesce 后发起新连接。
- Drain 超时。
- 重复 Stop。
- Serve 在 Ready 后异常退出。

#### 验收标准

- 任一失败后端口可以立即重新绑定。
- 无残留 SessionHub 记录。
- 无残留 listener 和 goroutine。
- Runtime 错误包含 `tcp/ws/kcp` 来源。
- 排空期间已有请求继续完成，新会话被拒绝。

#### 执行注意点

- Start 部分失败的清理属于当前组件责任。
- Stop 错误不能只写日志。
- 三种传输必须使用相同排空语义。
- 回滚本身失败时必须保留原始启动错误。

### V3-P0-05 Redis、XORM、Nacos、Gamedata 生命周期

#### Redis

执行：

- Manager 增加 `Close(context.Context) error`。
- 初始化多个实例时记录成功列表。
- 中途失败立即逆序关闭成功实例。
- 删除无意义的重复 Load。
- 连接池关闭错误返回 Runtime。

验收：

- 第二个实例失败时第一个实例已经关闭。
- 重复 Close 不 panic、不重复释放。
- 日志不包含密码。

#### XORM/MySQL

执行：

- OrmManager 增加 Close。
- Ping、SyncTables、注册失败时立即关闭 Engine。
- mysqlsvr Stop 同时关闭异步 Manager 和 ORM Engine。
- 完整 DSN 不进入日志或 error。

验收：

- 初始化中途失败无残留数据库连接。
- 重复 Stop 幂等。
- 连接池指标在 Stop 后归零。

#### MySQL 异步 Worker

执行：

- 删除业务 package `init()` 中的 worker 启动。
- 表注册改为显式 `RegisterTables()`。
- Worker Pool 改为 Component：
  - Start 创建 worker。
  - Quiesce 停止接收新任务。
  - Drain 等待队列。
  - Stop 取消并 Wait。
- 新代码使用 Go 风格命名。

验收：

- import package 不产生 worker。
- mysqlsvr Drain 后队列为 0。
- Stop 后所有 worker 已退出。
- 排空期间新任务被明确拒绝。

#### Nacos/Gamedata

执行：

- Nacos client 创建返回 `(client, error)`。
- 保存每个 ListenConfig 的 DataID 和 Group。
- Stop 调用 CancelListenConfig。
- Gamedata 更新流程固定为：
  1. 读取候选数据。
  2. 完整解析。
  3. 完整校验。
  4. 构造不可变快照。
  5. 原子替换。
  6. 通知版本变化。
- 解析失败保持旧快照。

验收：

- Nacos 启停 100 次无监听 goroutine 线性增长。
- Gamedata 并发读取和热更通过 race。
- 热更失败后旧数据仍可访问。
- 取消监听失败能够通过 Stop error 观察。

#### 执行注意点

- 不引入通用反射式资源容器。
- 有资源的依赖使用具体 Component。
- Stop 必须容忍 Start 只完成部分步骤。
- 不在日志中记录完整远端配置内容。

### V3-P0-06 Web HTTP/gRPC 生命周期

#### 执行步骤

1. 将包级 HTTP server 改为实例对象。
2. Start 中先 `net.Listen`，成功后再启动 Serve goroutine。
3. Start 返回 nil 即代表端口已绑定且可服务。
4. 暴露 RuntimeErrors。
5. 配置并校验：
   - ReadHeaderTimeout。
   - ReadTimeout。
   - WriteTimeout。
   - IdleTimeout。
   - MaxHeaderBytes。
   - MaxBodyBytes。
6. Quiesce 顺序：
   - readiness 失败。
   - gRPC health 设置 `NOT_SERVING`。
   - 停止接收新 HTTP/gRPC 请求。
   - 等待在途请求。
7. Stop 在 context 超时后执行强制关闭。
8. reflection 仅在 debug 配置下启用。
9. Redis、签名、敏感词、HTTP、gRPC 拆为资源边界清晰的组件。

#### 验收场景

- HTTP 端口冲突。
- gRPC 端口冲突。
- Serve 在启动后返回错误。
- 慢 Header 请求。
- 超大 Body。
- 长请求期间执行 Quiesce。
- gRPC streaming 期间 Drain。
- Shutdown context 超时。

#### 验收标准

- 端口冲突时服务不能进入 Ready。
- Quiesce 后 1 秒内 `/readyz` 返回非 200。
- 已接受请求在 drain timeout 内完成。
- 超时后进程能在 stop timeout 内退出。
- HTTP Serve 异常会触发 App Drain/Failed。

#### 执行注意点

- HTTP 启动成功不能仅表示 goroutine 已创建。
- Shutdown error 必须向 Runtime 返回。
- 强制 Stop 只能发生在优雅关闭超时之后。
- 默认超时值必须兼容现有业务，再由生产配置收紧。

### V3-P0-07 代码风格和文档一致性

#### 执行步骤

1. 新增 `.gitattributes`，明确 Go、Markdown、Shell 行尾。
2. 不执行一次性全仓格式化。
3. 本轮触及的非生成 Go 文件必须：
   - gofmt。
   - 中文 GoDoc。
   - UID、IP、ID 命名符合规范。
   - 错误使用 `%w`。
   - 不使用组件内 Fatal。
   - 不新增裸 goroutine。
4. 清理生产代码中的阶段性历史注释：
   - `P0-xx/P1-xx`。
   - “方案 B”。
   - “roadmap”。
5. 历史过程迁移到文档，代码只保留不变量和设计原因。
6. protoc 插件输出前执行 `go/format`。
7. 不手工修改生成代码。

#### 验收方法

```bash
git diff --check
./main.sh check-genproto --full
```

#### 验收标准

- 本轮触及文件 gofmt 无输出。
- 生成两次代码没有 diff。
- 非生成生产代码不再出现阶段编号注释。
- 没有因行尾造成的全文件无语义变更。

#### 执行注意点

- 风格修改单独提交。
- 不顺便清理与本轮无关的旧代码。
- 已公开名称本轮不做破坏性重命名。

## 5. P1：生产能力与简洁性

### V3-P1-01 Admission Control 与过载保护

#### 配置模型

在 grouped runtime/capacity 配置下增加：

- `max_connections`
- `max_unauthenticated_connections`
- `connection_rate`
- `login_rate`
- `max_inflight`
- `max_inflight_per_method`
- `overload_mode`: `off`、`shadow`、`enforce`

默认保持兼容，但生产配置必须显式设置。

#### 执行步骤

1. connsvr listener 接收连接前检查总连接数。
2. 未认证 Session 使用独立上限。
3. 登录使用全局、IP 两级限速。
4. SSRPC 增加 admission middleware。
5. 默认不等待，超过并发上限立即拒绝。
6. 必须排队时，队列必须有长度和等待超时。
7. 定义统一 `ErrOverloaded`。
8. 增加指标：
   - active connections。
   - unauthenticated connections。
   - inflight。
   - queue depth。
   - rejected total。
   - admission wait duration。
9. Prometheus 标签只使用 service、method、reason 等低基数值。

#### 验收标准

- 压力超过额定容量 150% 时内存和 goroutine 不持续上涨。
- 过载时快速失败，不通过无限排队维持表面成功率。
- 被接受请求的 P99 不超过正常负载的 2 倍。
- shadow 与 enforce 决策数量可以对账。
- Quiesce 优先于普通 admission 判断。

#### 执行注意点

- 不破坏 mainsvr、roomcentersvr 的 UID/房间串行模型。
- 不使用每个 UID 一个永久 limiter。
- IP 限制考虑 NAT，不使用不可配置的固定小上限。
- 预期流控拒绝不全部记录 Error 日志。

### V3-P1-02 SessionHub 单路径化

#### 执行步骤

1. TCP、WS、KCP 内部始终持有非 nil SessionHub。
2. 旧构造函数内部创建独立 Hub。
3. 新构造函数显式接收共享 Hub。
4. `SetHub` 标记 Deprecated，只允许 Start 前使用。
5. 删除本地 Session map、锁和 `hub != nil` 双分支。
6. RemoteAddr 字符串作为完整地址来源。
7. 数值 IP 明确只支持 IPv4。
8. 新增严格 BusID/IP 解析接口，非法输入返回 error。
9. 旧无错误解析接口保留一个弃用周期。

#### 验收标准

- 三种传输通过相同 SessionHub 契约测试。
- lookup 保持 0 allocation。
- BindReplace 不发生 Session 泄漏。
- IPv4、IPv6、非法地址行为与文档一致。
- Bind、Kick、Replace、Quiesce、Stop 通过 race。

#### 执行注意点

- 兼容构造函数也必须进入同一 Hub 实现，不能保留第二套 map。
- 不通过对象池优化低频登录路径，除非容量测试证明必要。

### V3-P1-03 服务装配简化

#### 设计规则

- `main.go` 保持当前显式 Run 形式。
- 不增加 `runtime.Main` 门面。
- `NewApp` 只负责：
  1. 声明配置加载。
  2. 构造具体组件。
  3. 按依赖顺序注册。
  4. 返回 App。
- 不创建通用 DI 容器。
- 无资源初始化才允许使用 FuncComponent。

#### 执行步骤

1. 每个服务整理为：
   - config loader。
   - dependency components。
   - SSRPC registration。
   - runtime components。
   - `MustRegister` 顺序。
2. 删除陈旧 application/bootstrap 注释。
3. SSRPC 统一使用生成 Registry API。
4. 组件名统一为稳定的 snake_case。
5. Handler 注册失败必须发生在监听器启动前。
6. 启动顺序注释只说明必要依赖。

#### 验收标准

- 每个服务 NewApp 可以独立构造。
- 重复 Handler 注册在 Start 阶段失败。
- 注册失败时没有端口、连接池或 worker 启动。
- 六服务均有启动和停止顺序测试。

### V3-P1-04 配置不可变与全局状态收敛

#### 执行步骤

1. 新增各服务返回配置值的读取接口。
2. 完整 Normalize、Validate 成功后才发布。
3. NewApp 捕获服务局部配置。
4. 少量业务层全局配置访问改为构造参数或只读 source。
5. 除 gamedata 外不建设运行配置通用热更。
6. `appconfig.Store`、`WithReload`、仅测试使用的 Module Registry 标记 Deprecated。
7. 下一破坏性版本删除无生产用途 API。

#### 验收标准

- 配置失败时无部分发布。
- Start 后运行配置不被原地修改。
- 配置并发读取通过 race。
- 需要重启的字段变化只报告字段名，不记录值。
- 六服务不新增 `gconf.*Cfg` 直接访问点。

#### 执行注意点

- 不为“未来可能热更”增加通用抽象。
- map、slice 发布前必须深拷贝或保证不可变。
- 兼容全局变量在弃用期内只能只读。

### V3-P1-05 显式 Driver 与依赖裁剪

#### 执行步骤

1. RabbitMQ 保持生产主线。
2. 检查六服务最终依赖：
   - websvr 不包含 MQ SDK。
   - RabbitMQ 服务不包含 NATS、Kafka、NSQ、RocketMQ SDK。
3. 当前版本标记包级 Driver 注册、`CreateBus`、driver `init()` 路径为 Deprecated。
4. 其他消息驱动进入独立兼容清单。
5. 正式支持的驱动必须拥有：
   - Start。
   - RuntimeErrors。
   - Quiesce。
   - Drain。
   - Stop。
   - 断线重连测试。
6. 下一主版本将非 RabbitMQ 驱动迁入独立 Go module。

#### 验收方法

```bash
go list -deps ./cmd/websvr
go list -deps ./cmd/connsvr
go mod graph
```

#### 验收标准

- websvr 不链接 MQ SDK。
- RabbitMQ 服务不链接其他 MQ SDK。
- 正式支持驱动均进入 CI 集成矩阵。

#### 执行注意点

- “二进制未链接”和“根模块不依赖”分别验证。
- 不通过 build tag 隐藏无法编译的驱动。
- 未进入测试矩阵的驱动不得标记为正式支持。

### V3-P1-06 容量工具升级

#### 当前问题

- TCP Write 成功不能证明服务端登录成功。
- 必须等待并校验真实登录响应。
- 心跳 Write 成功不能证明服务端已经处理。
- 当前工具缺少请求延迟、断线率、CPU、RSS、FD 和恢复时间。

#### 执行步骤

1. 复用 tester Session 和真实协议。
2. 每个请求按序列号关联响应。
3. 支持以下阶段：
   - connect。
   - login。
   - ramp-up。
   - steady。
   - drain。
   - reconnect/recovery。
4. 支持多压测进程和多压测机分片。
5. 输出 JSON 原始数据。
6. 自动生成容量矩阵 Markdown。
7. 客户端记录自身 CPU、内存和网络使用。
8. 服务端采集：
   - CPU。
   - RSS。
   - GC pause。
   - goroutine。
   - FD。
   - Session。
   - Bus backlog。
   - Redis/MySQL pool。
   - P50/P95/P99/P999。
   - disconnect/reconnect。
   - overload rejection。

#### 容量阶梯

| 阶段 | 长连接 | 登录速率 | 稳态消息 |
|---|---:|---:|---:|
| C1 | 1,000 | 100/s | 500/s |
| C2 | 3,000 | 200/s | 1,500/s |
| C3 | 5,000 | 300/s | 2,500/s |
| C4 | 10,000 | 500/s | 5,000/s |

执行规则：

- 每档 Ramp 后稳定运行 30 分钟。
- C1 通过后才能执行 C2，依次升级。
- 任一档失败先采集 profile，不直接修改代码。
- 压测客户端资源超过 60% 时必须扩展压测机，避免客户端成为瓶颈。

#### C4 验收标准

- 连接成功率 ≥ 99.9%。
- 业务请求成功率 ≥ 99.9%。
- 框架链路 P99 ≤ 50ms。
- CPU ≤ 分配核心的 70%。
- 最后 15 分钟 RSS 增长 < 5%。
- GC pause P99 ≤ 20ms。
- goroutine、FD 无持续增长。
- Drain 后 goroutine、FD 回到基线 ±2%。
- readiness 在 1 秒内关闭。
- 已接受请求不丢失。
- 30 秒 drain timeout 内完成排空。

#### 输出物

- `docs/benchmarks/capacity-matrix.md`。
- 带 commit、Go 版本、Linux 内核、CPU、内存和参数的原始结果。

### V3-P1-07 基于证据的性能优化

#### Packet 编解码

执行：

- 增加 `AppendTo/MarshalTo`。
- 发送热路径写入已有缓冲区。
- 保留 `ToBytes` 兼容接口。

目标：

- CSPacketHeader 发送热路径达到 0 allocation。
- From 路径继续保持 0 allocation。

#### TransactionMgr

修改前必须采集：

- CPU profile。
- alloc profile。
- mutex profile。
- block profile。
- scheduler trace。

只有 profile 证明后才处理：

- 请求包装分配。
- Channel 任务对象。
- Context 或 Closure 分配。
- Shard 查找。
- 回包对象生命周期。

不得破坏：

- UID/房间串行。
- 相同 key 顺序。
- panic 隔离。
- timeout/cancel。
- drain。

#### SessionHub

- lookup 已达到低延迟，不优先重写。
- BindReplace 仅在登录 Ramp 失败时优化。
- 不为低频路径引入复杂对象池。

#### 性能合并门禁

- 同机器、同 Go 版本、同 GOMAXPROCS。
- 前后各至少执行 10 次。
- 使用 benchstat。
- 中位数回退不得超过 5%。
- 0-allocation 路径不得新增分配。
- 没有 profile 或 benchmark 的性能修改不得合并。

## 6. P2：条件执行项

### V3-P2-01 gnet v2

只在 Linux A/B 分支验证。

迁移门槛：

- 吞吐提升 ≥ 10%；或
- CPU/RSS 降低 ≥ 15%；并且
- C4、race、断线恢复、Drain 全部通过。

未达到门槛则保持 Deferred。

### V3-P2-02 PGO

只使用代表性生产 CPU profile。

启用门槛：

- 容量矩阵稳定提升 ≥ 5%。
- 无明显长尾恶化。
- 无 profile 时构建可以回退。
- profile 不包含敏感业务数据。

### V3-P2-03 Agones Adapter

当前继续使用 VM/Ansible，保留 Agones 适配边界，不提前引入 SDK。

明确进入 Agones 后实施：

- Ready。
- Health。
- Reserve。
- Allocate。
- Shutdown。
- WatchGameServer 状态确认。
- SIGTERM 与 Sidecar Shutdown 统一排空。

验收场景：

- Pod 驱逐。
- 滚动发布。
- 分配中断。
- Sidecar 重启。
- Drain 超时。
- 远端状态延迟和失败。

### V3-P2-04 下一主版本破坏性清理

在一个完整弃用周期后删除：

- Driver `init()` 注册。
- 全局 Driver Registry。
- Legacy Dispatcher/TransMgr 注册。
- SessionHub 可选分支。
- `SetHub`。
- 无生产用途 Module Registry。
- 通用 Reload Store。
- 旧配置加载器。
- 无错误返回的 BusID/IP 解析接口。

### V3-P2-05 基础依赖现代化

独立执行：

1. Nacos v1/v2 统一。
2. 旧 protobuf API 迁移到新 API。
3. XORM 新版本或替代方案评估。
4. Redis 客户端 context 和连接池能力评估。
5. 可选 MQ 驱动迁出根模块。

每项必须提供：

- 兼容矩阵。
- 数据和协议一致性测试。
- 故障恢复测试。
- 性能对比。
- 回滚方案。

## 7. 分批发布与回滚

### 7.1 发布顺序

1. infosvr。
2. mysqlsvr。
3. mainsvr。
4. roomcentersvr。
5. connsvr。
6. web_svr。

每个服务依次经过：

1. 开发环境启动/停止循环。
2. 集成回归。
3. 故障注入。
4. 10% 实例灰度。
5. 50% 灰度。
6. 全量。

### 7.2 灰度观察指标

- 启动成功率。
- Ready 耗时。
- RuntimeError 次数。
- Drain 耗时。
- 强制 Stop 次数。
- goroutine、FD、RSS。
- 请求错误率。
- P95/P99。
- Bus backlog。
- Redis/MySQL pool saturation。

### 7.3 回滚条件

任一条件满足即回滚：

- 错误率上升超过 0.5%。
- P99 上升超过 20% 且持续 5 分钟。
- RSS 持续增长。
- Drain 超过配置时间。
- Session 无法释放。
- RuntimeError 无法定位组件。
- 中间件连接池无法恢复。
- 新版本无法在 stop timeout 内退出。

Admission Control 可通过配置从 `enforce` 回退到 `shadow/off`，不要求立即回滚二进制。

## 8. 提交与执行纪律

每个任务遵循：

1. 先添加失败测试或复现脚本。
2. 实施最小修改。
3. 运行局部测试。
4. 运行全量测试。
5. 更新对应文档和基准。
6. 独立提交。

禁止：

- 一个提交同时升级依赖、重构生命周期和格式化代码。
- 全仓 gofmt 或换行符重写。
- 手工修改生成代码。
- 新增无上限 goroutine、channel、队列或 cache。
- 在 Component 内调用 Fatal。
- 忽略 Stop、Drain、Serve 返回错误。
- 以日志代替错误传播。
- 没有数据就声称性能提升。
- 为未来可能需求引入通用 DI、插件系统或配置平台。

## 9. 阶段完成门禁

### P0 完成

- build、vet、unit、integration、race 全绿。
- govulncheck 可达漏洞为 0。
- 生命周期故障注入全部通过。
- 无凭据日志。
- 无已知资源泄漏。
- 文档链接无失效。
- 本轮代码符合 [代码风格规范](STYLE.md)。

### P1 完成

- Admission Control 支持 `shadow/enforce`。
- SessionHub 生产路径唯一。
- 配置以不可变快照使用。
- RabbitMQ 按需依赖得到证明。
- C1～C4 容量矩阵完成。
- C4 达到 10,000 连接 SLO。
- 所有性能修改有 benchstat/profile 证据。

### P2 完成

- 只实施达到量化门槛的升级。
- 下一主版本删除项有迁移文档。
- Agones、gnet、PGO 未满足条件时明确标记 Deferred。

## 10. 默认决策

- 兼容策略：一个版本的弃用过渡。
- 主消息驱动：RabbitMQ。
- 生产部署：当前 VM/Ansible，保留 Agones 边界。
- 配置策略：启动配置不可变，仅 gamedata 热更。
- 容量目标：单 connsvr 10,000 长连接。
- 生产基准环境：固定 Linux 机器。
- 架构原则：优先删除双路径、补齐生命周期，不增加通用抽象。

