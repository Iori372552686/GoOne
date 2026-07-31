# 可观测性与运行 SLO

> V4 P1-05：定义 GoOne 生产运行的可观测性契约、指标命名/标签规范、关键告警与 SLO。
> 目标是让运维仅凭指标与采样日志即可定位「过载、MQ 断线、Drain 超时、池饱和」等事件，
> 且 10,000 连接过程中 Prometheus time series 数量不随 UID/连接数线性增长。

## 1. 指标命名与标签规范

### 命名

- 统一前缀 `goone_`。
- 单位进入名称：耗时 `_seconds`（histogram/counter）、字节 `_bytes`、计数 `_total`。
- Gauge（瞬时值，如连接数、池大小）不加 `_total`。

### 标签（仅允许有界值）

允许的标签维度：`service`、`component`、`method`、`transport`、`reason`、`decision`、
`gate`、`direction`、`kind`、`cmd`、`result`、`state`、`role`、`db`、`instance`、`event`。

其中：
- `cmd` 来自 `g1_protocol.CMD(cmd).String()`，是协议命令枚举（有界），缺失归一为 `UNKNOWN`。
- `instance`/`db` 是配置的 Redis/MySQL 实例索引（小固定池），非每请求值。

**禁止**作为标签：UID、IP、request ID、连接 ID、完整错误文本、URL、Token。
这些高基数维度会让 time series 随连接/用户数线性膨胀，拖垮 Prometheus。

### 基数不变量

- 10,000 连接、百万级 UID 下，time series 总数应保持为常量级（仅随 `cmd` 枚举数 ×
  指标数增长，不随连接数增长）。
- 任何新增标签上线前须确认其值域有界。

## 2. 关键指标清单（按子系统）

| 子系统 | 指标 | 类型 | 标签 | 用途 |
|---|---|---|---|---|
| lifecycle | `goone_lifecycle_state` | gauge | state | Ready/Draining/Failed 状态 |
| lifecycle | `goone_component_start_duration_seconds` | hist | component | 组件启动耗时 |
| lifecycle | `goone_component_start_failures_total` | ctr | component | 启动失败 |
| lifecycle | `goone_drain_duration_seconds` | hist | — | 排空耗时 |
| lifecycle | `goone_drain_timeouts_total` | ctr | — | 排空超时 |
| lifecycle | `goone_config_reload_total` | ctr | result | gamedata 热更结果 |
| gateway | `goone_gateway_connections` | gauge | kind | 连接/会话计数 |
| gateway | `goone_gateway_events_total` | ctr | transport,event | 连接生命周期事件 |
| admission | `goone_admission_decisions_total` | ctr | gate,decision,reason | 过载准入决策 |
| router | `goone_router_messages_total` | ctr | direction,kind,cmd,result | 消息收发计数 |
| router | `goone_router_message_duration_seconds` | hist | direction,kind,cmd | 链路耗时（P50/P95/P99/P999） |
| router | `goone_router_message_errors_total` | ctr | direction,kind,cmd | 发送失败 |
| router | `goone_router_messages_in_flight` | gauge | direction,kind,cmd | 在途消息 |
| redis | `goone_redis_commands_total` | ctr | instance,cmd,result | Redis 命令计数 |
| redis | `goone_redis_command_duration_seconds` | hist | instance,cmd | Redis 耗时 |
| redis | `goone_redis_commands_in_flight` | gauge | instance,cmd | Redis 在途 |
| xorm | `goone_xorm_pool_connections` | gauge | db,role,state | MySQL 连接池占用 |
| xorm | `goone_xorm_ping_errors_total` | ctr | db | MySQL 健康检查失败 |
| scheduler | `goone_task_duration_seconds` | hist | — | 周期任务耗时 |
| scheduler | `goone_task_skipped_total` | ctr | — | 重入跳过 |

> RabbitMQ bus 的运行期断线经 `RuntimeErrors` 上报，触发标准 Drain/Failed；
> 断线/重连事件见 lifecycle 与 gateway 指标。

## 3. 告警定义

| 告警 | 触发条件（PromQL 概要） | 严重度 | 含义 |
|---|---|---|---|
| **Ready 耗时过高** | `histogram_quantile(0.95, goone_component_start_duration_seconds_bucket) > 30s` | warning | 启动慢，影响滚动发布 |
| **RuntimeError** | `goone_lifecycle_state == 4`（Failed） | critical | 运行期不可恢复错误，需介入 |
| **Drain 超时** | `increase(goone_drain_timeouts_total[5m]) > 0` | warning | 排空未在超时内完成，可能有连接被强制关闭 |
| **Drain 耗时过高** | `histogram_quantile(0.99, goone_drain_duration_seconds_bucket) > 30` | warning | 排空慢 |
| **过载拒绝** | `increase(goone_admission_decisions_total{decision="reject"}[1m]) > 0` | warning | 流量超容量，已开始拒绝 |
| **强制 Stop** | `goone_gateway_events_total{event="force_stop"}` 上升 | warning | 连接被强制关闭（排空未完成） |
| **MQ 断线** | `goone_lifecycle_state` 进入 Draining 且伴随 RuntimeError | critical | RabbitMQ 断线触发 Drain |
| **Redis 池饱和** | `goone_redis_commands_in_flight / goone_redis_pool_max > 0.8` | warning | Redis 连接池接近上限 |
| **MySQL 池饱和** | `goone_xorm_pool_connections{state="in_use"} / goone_xorm_pool_connections{state="max_open"} > 0.8` | warning | MySQL 连接池接近上限 |
| **MySQL 不可达** | `increase(goone_xorm_ping_errors_total[1m]) > 0` | critical | 数据库健康检查失败 |
| **链路 P99 高** | `histogram_quantile(0.99, goone_router_message_duration_seconds_bucket) > 0.05` | warning | 框架链路 P99 > 50ms |

## 4. 结构化日志规范

- 统一经 `lib/api/logger` 门面，禁止 `log`、`fmt.Print*`、裸 zap。
- 级别：Debug=开发排查；Info=状态变更（低频）；Warning=可自愈异常；Error=需关注失败；
  不用 Fatal（进程退出由启动层决策）。
- **采样保留**：过载流控、MQ 断线等高频事件采样后仍须保留**首个错误、周期摘要、恢复事件**。
- 预期流控（如 admission shadow reject）不记 Error 级别。
- **禁止**打印完整消息体、Token、密码、Authorization、配置正文；只打长度与关键 ID。

> 当前 logger 为 printf 风格（Sugared zap + console encoder）。`service/component/phase/reason`
> 结构化字段化属后续架构演进项，本轮不强制；新增关键日志建议在消息中包含 component 与
> phase 关键词以便检索。

## 5. tracing 规范

- tracing 不记录 Header、Token、完整 Body 或配置正文。
- 透传 `TraceID`/`SpanID`（见 `sharedstruct.TraceContext`）用于全链路关联，不作为指标标签。

## 6. SLO（生产容量目标，对应 P1-03 C4）

| SLO | 目标 | 度量指标 |
|---|---|---|
| 连接成功率 | ≥ 99.9% | `goone_gateway_events_total{event="connect_ok"} / total` |
| 登录/业务成功率 | ≥ 99.9% | `goone_router_messages_total{result="ok"} / total` |
| 框架链路 P99 | ≤ 50ms | `goone_router_message_duration_seconds` |
| GC pause P99 | ≤ 20ms | go_gc_duration_seconds |
| readiness 关闭 | ≤ 1s | Ready→Draining 状态翻转时延 |
| Drain 完成 | ≤ 30s | `goone_drain_duration_seconds` |
| Drain 后资源回收 | goroutine/FD 回到基线 ±2% | runtime metrics |
