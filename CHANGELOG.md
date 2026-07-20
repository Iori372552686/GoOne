# Changelog

本文件记录 GoOne 的重要变更。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### ⚠️ Breaking

- **SS 协议头升级 v2（54B → 86B）**：追加 `TraceID(16B)/SpanID(8B)/DeadlineUnixMs(8B)`
  实现全链路 trace 透传与级联超时。新旧节点头长不一致，**集群必须整组发版，
  不支持滚动混布**。
- **核心生命周期现代化（P0）**：删除 `lib/service/application`（全局单例 + 10ms Tick
  + Fatal）与 `lib/service/bootstrap`（ServiceApp Hook + admin）。六个服务全部迁移到
  `lib/service/runtime.App.Run` + Component 生命周期。详见
  [`docs/architecture_review_2026-07-v2.md`](docs/architecture_review_2026-07-v2.md)。
  **配置/协议/持久化格式不变**，但服务装配方式改变（`busapp.New` → `runtime.App` +
  Component）。

### Added — 核心现代化（P0 + P1）

- **`lib/service/runtime`**：统一 App.Run + Component/Quiescer/Drainer 生命周期、
  Ready/Allocated/Draining 状态机、StateObserver、AdminComponent（statez/healthz/
  readyz/components/metrics，默认 loopback）、Module/Registry 装配、Gateway
  Quiesce/Drain 契约 + SessionTracker。
- **`lib/service/appconfig`**：不可变配置 Store[T] + 安全局部重载（白名单/
  restart_required/深拷贝防别名/原子交换）。
- **`lib/service/scheduler`**：受生命周期管理的周期 Task（NonOverlap 跳过、Stop join
  在途、panic 不杀 loop）。mainsvr 角色心跳（1 分钟）、roomcentersvr 房间 Tick（5s）+
  Persist（10s）。
- **`lib/service/ssrpc.Registry`**：Binding 批量原子注册 + Dispatcher.Seal 只读 map，
  DispatchWS 热路径无锁（2.84ns vs 旧 10.18ns，-72%）。
- **`lib/service/bus.DriverRegistry`**：显式 Driver 描述符装配（重复报错、未链接报错）。
- **`lib/util/bufpool.Acquire/Release`**：Buffer Lease（0-alloc），tcp/ws/kcp 写队列
  全部迁移（7.5ns/0alloc vs 旧 Get/Put 18.7ns/1alloc）。
- **网关排空**：tcp/ws/kcp listener 字段化 + Quiesce（停接新连接保既有）/Stop（强制
  关闭全部），connsvr 实现完整 Quiescer。
- **可观测性指标**：`goone_lifecycle_state`/`component_start_duration`/
  `drain_duration`/`task_duration`/`task_skipped`/`config_reload`。
- **CI**：race 扩充 runtime/appconfig/scheduler/bus/bufpool；lint 去除
  continue-on-error（阻断合并）；旧生命周期 API 静态扫描；docs link check。
- **STYLE.md**：第 2 节注释统一中文（硬性规则）；第 11 节 P0 迭代规范。
- 真实环境联调验证通过（六服务 Ready + tester ALL PASSED + admin 端点）。

### Removed

- `lib/service/application`（全局单例 AppInterface + 10ms Tick 循环 + Init Fatal）。
- `lib/service/bootstrap`（ServiceApp Hook + admin server + busapp 装配层）。
- 全局 10ms Tick（替换为精确周期 Task）。
- `runtime.GOMAXPROCS(NumCPU()+1)` 调用（Go 1.25 自动管理）。
- 启动路径的 `logger.Fatalf`。

### Added

- `lib/api/gerr`：统一框架错误模型（Code + Reason + Message + 错误链），`ssrpc.ToErrorCode` 已对接。
- **全链路 trace 与级联超时**：connsvr 生成 root trace，`Transaction.Call*` 自动透传 trace/
  收缩 deadline，接收端丢弃超期请求（`dropped_deadline_exceeded`），ssrpc 日志自动带 trace_id。
- **bus 驱动插件化**：五个 MQ 实现移入 `bus/driver/<name>`（database/sql 风格 blank import），
  `driver/all` 聚合包；不用 bus 的服务不再链接 MQ SDK（websvr 依赖图 MQ 包 68→0）。
- `router.Router` 结构体化（`New()/Default()`），包级 API 兼容不变，测试可注入。
- `net_mgr.GatewayServer` 统一网关接口（TCP/WS/KCP 三传输实现）+ `gatewayTransport` 后端抽象。
- **KCP 网关转正**：流模式 + CSPacket 粘包拆包 + 池化写缓冲 + 完整会话层
  （绑定/下行/踢人/多地登录/心跳登出），connsvr `runtime.kcp_port` 启用。
- **gnet 事件驱动 TCP 后端**：epoll/kqueue 无每连接 goroutine，事件循环内拆包，
  connsvr `runtime.tcp_impl_type: gnet` 切换；下行经 AsyncWrite。
- connsvr 三传输统一客户端包处理（`handleClientPacket`）与下行回退链 TCP→WS→KCP。
- 遗留删除：`beego_ws.go`、Beego 风格 `lib/web/rest`（`Cors` 迁至 `web_gin`）。
- `bootstrap/busapp`：bus 服务标准装配层。
- 可观测性：`docs/observability/`（Grafana dashboard 模板 + 指标/告警/trace 指南）。
- legacy 配置字段 Deprecated 标记与启动告警（两个版本后删除）。
- 风格与工程：`docs/STYLE.md`、`CONTRIBUTING.md`、PascalCase 文件重命名、拼写修正。
- `lib/service/bootstrap/busapp`：bus 服务标准装配层，五个 bus 服务 app.go 迁移为声明式 Options。
- `lib/util/bufpool`：通用字节缓冲池，覆盖网关写路径与 bus 发送帧。
- `IBus.Healthy()` 与 `router.ReadyCheck()`：MQ 断连期间 `/readyz` 返回 503 自动摘流。
- `bootstrap.Options.ReadyCheck`：自定义就绪探针注入点。
- `RoleMgr.FlushAllToDB`：优雅停机时全量落盘在线角色。
- CI（GitHub Actions：build/vet/test/race/check-genproto/lint 增量）、`.golangci.yml`。
- 性能基线与优化对比报告：`docs/benchmarks/baseline.md`。
- 文档：架构评审（`docs/architecture_review_2026-07-v2.md`）、迭代计划（`docs/optimization_roadmap.md`）、
  代码风格规范（`docs/STYLE.md`）、`CONTRIBUTING.md`。

### Changed

- **性能**：网关热路径日志治理 + Timer 复用 + 头编码栈上化 + 写缓冲/bus 帧池化 + 每包 goroutine 收敛，
  TCP 回环路径 allocs/op 20→1、B/op 1124→25、延迟约 -28%（详见 benchmarks 报告）。
- 网关 TCP/WS/KCP 的包处理改为读协程内同步执行：恢复同连接消息顺序、提供天然背压。
- `TransactionMgr.ProcessSSPacket` 入队改为有界（3s 超时 + 丢弃指标），不再无限阻塞 bus 消费链。
- 角色心跳过期淘汰改经事务串行执行（`SelfLogoutSender` 投递 Logout），消除 Tick 与 handler 的并发竞争。
- `safego` panic 恢复不再触发进程级 Fatal，改为 Error 日志 + Prometheus 计数。
- `Transaction.waitRsp` 超时/关闭返回语义化错误（`gerr.ErrTimeout`/`gerr.ErrClosed`）。
- NATS/Kafka bus 接收侧去除冗余拷贝；Kafka reader 失败自动重连；五实现重连循环去 `time.After`。
- 网络工厂未实现分支（gev/gnet/beego）由静默成功改为显式报错。
- 文件重命名（snake_case）：`lib/api/http_sign`、`lib/web/rest` 全部 PascalCase 文件；
  拼写修正 `actvity_task.go`→`activity_task.go`、`ai_creat.go`→`ai_create.go`
  （函数 `OnAiCreatRoom`→`OnAiCreateRoom`、`OnMainCreatRoom`→`OnMainCreateRoom`）。

### Fixed

- mysqlsvr：`MysqlMgr` 从未初始化导致 `UpdateRoleInfo/SearchRole` 必然失败（迁移至 OrmMgr）。
- xorm 读写分离：slave DSN 误用 master 端口。
- 网关 TCP/KCP 异步包处理引用可复用读缓冲的 data race。
- mainsvr 停机不落盘在线角色（write-behind 窗口内数据丢失）。
- WS 通道下行不通（下行仅走 TCP）；gin WS 升级后未触发 `OnConn`。
- `BroadcastByZone` 未过滤 zone；德州补房条件判断错误。
- `router.InitAndRun` 吞掉 `bus.CreateBus` 错误。
- 多个测试修复：崩坏的 safego/crypto/xorm/nacos 测试、缺可达性 skip 的集成测试、
  kcp/nsq 永不退出的测试、transaction Close 竞态、LRU Peek 断言反向。

### Removed

- `Role.SaveToDBIgnoreRsp`（fire-and-forget 落盘，被同步/串行路径替代）。
- 未使用的 OpenTelemetry 直接依赖（`go mod tidy`）。

---

<!--
发布流程（维护者）：
1. 将 Unreleased 内容移入新版本段落：## [vX.Y.Z] - YYYY-MM-DD
2. git tag vX.Y.Z && git push origin vX.Y.Z
3. GitHub Release 引用对应段落
-->
