# GoOne 代码风格规范

> 适用范围：整个仓库的新增与修改代码。存量代码按 [`optimization_roadmap.md`](optimization_roadmap.md) 4.2 分批清理，
> 不要求一次性重写，但**新代码必须遵守本规范**，CI 的 golangci-lint（`--new-from-rev`）只对增量生效。

## 1. 命名

### 1.1 包与目录

- 包名：全小写单词，不使用下划线、不使用复数（`bufpool`、`gerr`）。
- 目录名与包名一致；服务目录统一 `<name>svr` 连写风格（如 `roomcentersvr`）。
  存量的 `web_svr` 目录保留（改动成本大），新服务不得再用下划线。
- 实验性/未完成的实现放 `x/` 前缀目录或独立分支，不混入主干生产路径。

### 1.2 文件名

- 统一 snake_case：`role_mgr.go`、`pack_proc.go`。禁止 PascalCase 文件名。
- 后缀约定（自有约定，全仓一致执行）：
  - `*_i.go`：仅接口定义；
  - `*_impl.go` / `*_impl_<variant>.go`：接口实现；
  - `*_test.go` / `*_bench_test.go`：测试与基准。
- 拼写必须正确。发现拼写错误的标识符/文件名，修复时机：文件名随时可改；
  导出符号与生成代码路径的拼写错误在大版本边界统一修正。

### 1.3 标识符

- **新代码接口命名遵循 Go 惯例**：行为接口用 `er` 后缀（`Registry`、`Logger`），
  禁止新增 `I` 前缀接口（`IBus`、`IContext` 为存量兼容，逐步迁移）。
- 缩写词保持一致大小写：`ID`、`URL`、`HTTP`（`uid`→`UID` 在导出符号中）。
- 错误变量：包级哨兵错误用 `Err` 前缀（`gerr.ErrTimeout`）。

## 2. 注释

- **注释统一使用中文（硬性要求）**：包括导出符号的 godoc 注释、包文档
  （`// Package xxx ...`）、行内实现说明。导出符号注释仍以符号名开头
  （`// FlushAllToDB 同步把在线角色落盘……`）。godoc 工具对中文注释兼容良好。
  外部不可控的第三方生成代码（`api/gen/**`、`*.pb.go`）保持原样，不强制翻译。
- **注释解释“为什么”，而非复述代码做了什么**。能在代码里自表达的，不要写注释。
- 包级文档：每个 `lib/` 子包在主文件顶部写 `// Package xxx ...` 说明职责、用法与
  关键约束（见 `lib/service/runtime/doc.go`、`lib/service/appconfig/store.go`）。
- 导出符号（含常量、错误变量、结构体字段）必须有中文 godoc 注释，说明用途、调用
  约定或取值含义；字段注释写在行尾或上方均可。
- 禁止保留大段注释掉的代码——需要历史请依赖 git。

## 3. 错误处理

- **统一错误模型**：框架与业务错误使用 [`lib/api/gerr`](../lib/api/gerr/gerr.go)
  （或 ssrpc handler 中的 `ssrpc.E/Wrap`），禁止三种旧写法：
  1. 返回裸 `errors.New("timeout")` 这类无码错误穿越 RPC 边界；
  2. 业务 handler 直接写 `rsp.Ret.Code = ERR_X` 后 `return rsp, nil`
     （中间件会把失败统计为成功）——失败必须 `return nil, gerr.New(...)`；
  3. 新增 `uerror.UError` 用法（仅 cfgtool 存量保留）。
- 包装错误必须用 `%w` 或 `gerr.Wrap` 保留错误链；判断错误用 `errors.Is/As`，不比较字符串。
- `panic` 仅用于程序员错误（不可能到达的分支、init 期配置致命错误）；
  业务路径一律返回 error。goroutine 内用 `safego.Go`（recover + 指标，不杀进程）。

## 4. 并发

- 每连接/每实体的串行化优先复用框架机制（TransactionMgr serial key），
  禁止在 handler 与后台 goroutine 之间共享可变对象（参考 RoleMgr 淘汰路径的 `SelfLogoutSender` 模式）。
- channel 入队必须有界：快路径 `select+default`，慢路径带超时（参考 `SendToChan`）。
- 定时器：循环内复用 `time.Timer`（Stop+drain+Reset）；`select` 中禁用 `time.After`
  （用 `sleepOrStop` 模式）。
- 新增共享 map 优先考虑分片或 `sync.Map` 的适用性，说明选型理由。

## 5. 性能（热路径守则）

- 热路径（每消息/每帧执行的代码）：
  - 日志必须 Debug 级 + `logger.DebugEnabled()` 前置判断，避免参数装箱；
  - 禁止每消息 `make`——使用 `lib/util/bufpool` 或栈上数组（参考 SS 头编码）；
  - 禁止每消息 goroutine——串行执行或常驻 worker。
- 性能改动必须附 benchmark 前后对比（见 `docs/benchmarks/baseline.md` 流程），无数据不合并。

## 6. 日志

- 统一使用 `lib/api/logger` 门面，禁止直接使用 `log`、`fmt.Print*`、裸 zap。
- 级别语义：Debug=开发排查（可每消息）；Info=状态变更（连接建立、服务启停，低频）；
  Warning=可自愈异常；Error=需要关注的失败；不使用 Fatal（进程退出由启动层显式决策）。
- 禁止打印完整消息体/敏感字段（token、密码）；打长度与关键 ID。

## 7. 配置与接线

- 新配置只加 grouped 结构（`base_cfg.runtime` 等），禁止新增 legacy flat 字段。
- 启动配置视为**不可变值**：装配期读取后不再运行时改写；唯一允许的热更是 gamedata
  （经配置中心 watcher，每表 atomic.Value 原子替换）。
- 服务装配使用 `runtime.App + Component`（`runtime.MustNew` + `MustRegister`）；
  `bootstrap/busapp` 旧 API 已删除，不得重新引入。bus 服务经 `DriverRegistry`
  显式注册所需 MQ driver（通常仅 rabbitmq）。
- 依赖注入通过 app 装配层（Component 的 Start/Stop）；`globals.*` 仅限
  `src/<svr>/globals`，`lib/` 与 `module/` 禁止引用任何 globals。

## 8. 生成代码边界

- 禁止手改 `api/gen/**`、`game_protocol/protocol/*.pb.go`、`common/gamedata/repository/**/*.gen.go`；
  改 proto/生成器后执行 `go run ./tools/cmd/genproto` 并过 `./main.sh check-genproto`。

## 9. 测试

- 单元测试不依赖外部中间件；集成测试经 `lib/internal/itest.Require` 统一门控：
  未设 `GOONE_INTEGRATION=1` 时 `t.Skip`；**设置后中间件不可达必须 `t.Fatal`**
  （CI 环境下不得静默跳过，否则产生「用例数为 0 却通过」的虚假绿灯）。
- 修 bug 先写复现测试；性能项写 benchmark。
- 提交前本地过：`go build ./...`、`go vet -composites=false ./...`、
  `go test -count=1 ./...`、`go run ./tools/cmd/checkdocs ./docs`。

## 10. 提交与 CI

- 提交信息：`<type>: <summary>`，type ∈ feat/fix/perf/refactor/test/docs/chore。
- 合并前本地过：`go build ./...`、`go vet -composites=false ./...`、
  `go test ./lib/... ./src/...`、`golangci-lint run --new-from-rev=origin/dev`。

## 11. 生命周期与运行时（P0 核心现代化规范）

> 本节由 [`docs/architecture_review_2026-07-v2.md`](architecture_review_2026-07-v2.md)
> 与 [`docs/optimization_roadmap.md`](optimization_roadmap.md) P0 迭代落地，新代码
> 必须遵守。落地实现在 `lib/service/runtime`、`lib/service/ssrpc`、
> `lib/service/appconfig`、`lib/service/bus`。

### 11.1 Component 生命周期

- 每个运行时资源（监听器、bus consumer、事务分片、调度器、admin server）必须归属
  到**唯一一个** `runtime.Component`。
- 每个 goroutine 必须能由其所属组件的 Stop（或 Drain）context 取消并 join。
- `Component.Start` 返回 nil 时组件必须已可用；失败时由组件自行清理部分初始化，
  App **不**对 Start 失败的组件调用 Stop。
- `Component.Stop` 必须幂等、遵守 context、等待本组件 goroutine。
- 组件内**禁止** `os.Exit` / `logger.Fatalf`；致命决策由 `App.Run` 的调用方决定。
- 启停顺序：注册顺序即 Start 顺序；Quiesce / Drain / Stop 一律按注册**逆序**。
- 可选钩子 `Quiescer.Quiesce`（停止接新工作）与 `Drainer.Drain`（等待在途工作）：
  Quiesce 永远先于 Drain，两者先于 Stop。
- 禁止用 `time.After` 在 select 中判超时（见 §4）；用 `context.WithTimeout` 或复用
  `time.Timer`。

### 11.2 状态机

- 实例生命周期遵循规范状态：`Starting -> Ready -> (Allocated) -> Draining ->
  Stopping -> Stopped`，并在不可恢复错误时进入 `Failed`。
- 非法状态转换必须返回 `ErrInvalidStateTransition`，不得静默。
- `StateObserver`：Ready/Allocated 失败**必须回滚**到前一状态（失败的就绪闸门阻
  止服务）；Draining/Stopping/Stopped 的观察者错误只记录、**绝不阻断退出**。
- healthz/readyz 契约：Draining 时 healthz=200、readyz=**503**（必须在真正排空前即
  翻转）；Stopping/Stopped/Failed 两者皆 503。
- admin 端点：`/statez`、`/healthz`、`/readyz`、`/info`、`/components`。
  - admin IP 为空时**默认 127.0.0.1**，绝不绑所有网卡。
  - pprof 仅挂载在 admin server，且需 `admin.enabled && debug.pprof` 同时成立。
  - `statez`/`components` **不得输出**配置与连接凭据。

### 11.3 SSRPC 注册与不可变 Dispatcher

- 注册方法必须返回 `error`；重复注册必须失败（启动期暴露，而非运行期覆盖）。
- 批量注册（`ssrpc.Registry.Register`）原子提交：批次内或与既有状态任一冲突，
  **整批不提交**。
- `Dispatcher.Seal` 后不可变：cmd/ws/http 走只读 map，热路径
  （`DispatchWS`/`MountGin`/`MountGRPC`）**无锁**。
- Seal 必须发生在 bus consumer 与 listener Start **之前**。
- 遗留的 `RegisterCmd`（无返回值）保留供生成代码，但 Seal 后为记日志 no-op；
  新代码用 `RegisterCmdE` 或 `ssrpc.Registry`。
- 不改变既有 CMD、HTTP path、WS cmd、gRPC full method 的值。
- 不手改 `api/gen/**`；改 proto/生成器后跑 `go run ./tools/cmd/genproto` 并过
  `./main.sh check-genproto`。

### 11.4 不可变配置与安全重载

- 配置发布后不可变：`appconfig.Store[T]` 用 `atomic.Pointer[T]` 发布快照，读者
  （`Current()`）无锁读。
- Loader 每次返回全新对象；`Merger` 深拷贝 map/slice，**绝不别名**候选或旧快照。
- 热更新只走白名单（日志级别、tracing 相关字段）；其余字段（端口、MQ、DB、身份、
  容量等）差异上报为 `restart_required`，**不应用**。
- 重载先预构建候选（如 Trace Provider）成功后再原子交换；失败时旧状态不变，绝不出现
  部分提交（logger 新、tracing 旧、config 新）。
- 不打印完整配置与敏感字段（DSN、Password、Token、Secret、Headers）；只记字段名。

### 11.5 按需 Driver

- 不新增 `func init` 注册型 Driver/Handler/Module；不依赖包级全局 map 自注册。
- 各 bus driver 包导出 `func Driver() bus.Driver` 描述符；应用**显式**注册到
  `bus.DriverRegistry`（实例级，重复报错、未链接报错并列出可用项）。
- 未链接的 driver 是配置错误，返回明确 error，**不静默回退**。
- 默认二进制只链接 RabbitMQ；其他 driver 按需引入，缩小二进制与漏洞面。

### 11.6 性能与门禁

- 每项性能修改必须附 benchmark 前后对比（见 [`benchmarks/baseline.md`](benchmarks/baseline.md)），
  同机、同 Go 版本、同参数至少 10 次；核心吞吐中位数不得回退超过 5%，0-alloc 热路
  径不得引入新分配。
- 不把所有 goroutine 当性能问题：`TransactionMgr` 的在途事务 goroutine 模型以
  profile 证据（CPU/栈/分配占比）为决策门禁，未达阈值不改执行模型。

