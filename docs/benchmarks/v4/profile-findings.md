# 性能 Profile 发现与优化记录

> V4 P1-04：证据驱动的性能优化。
>
> **核心准则（计划硬约束）：**
> - profile 未显示为热点的候选文件**不得修改**。
> - 每个性能提交最多选择**一个**候选路径。
> - 同机器、同 Go、同 GOMAXPROCS、同日志级别下，中位数回退不超过 5%。
> - 原 0-allocation 路径不得新增分配。
> - TransactionMgr 不破坏 UID/房间串行、同 key 顺序、panic 隔离、cancel 和 Drain。
>
> 本文档记录每次 profile 的发现、选择的候选路径、优化前后 benchstat 对比与结论。
> 在取得 WSL Linux go1.26.5 的 profile 证据前，不修改任何候选文件。

## 基准复现命令

在 WSL Linux go1.26.5 下执行（与 PRE-02 基线同环境）：

```bash
# 合并门禁：before/after 对比
go test -run '^$' -bench . -benchmem -count=10 \
  ./lib/net/net_mgr ./lib/service/transaction \
  ./lib/service/router ./lib/service/bus/driver/rabbitmq > before.txt
# ... 实施最小修改 ...
go test -run '^$' -bench . -benchmem -count=10 \
  ./lib/net/net_mgr ./lib/service/transaction \
  ./lib/service/router ./lib/service/bus/driver/rabbitmq > after.txt
benchstat before.txt after.txt
```

## 候选路径（按计划优先级）

> 下列候选仅在**新 profile 显示为显著热点**时才允许修改。SSPacketHeader To/From
> 零分配路径、SessionHub Lookup、Sealed Dispatcher 查找已不再优先（除非重新成为热点）。

| # | 候选路径 | 文件 | 假设的热点来源 | 状态 |
|---|---|---|---|---|
| 1 | Admission CAS 竞争与 method counter | `lib/net/net_mgr/admission.go` | 高并发登录时 CAS 重试与 per-method 计数器竞争 | 待 profile |
| 2 | TransactionMgr 排队/channel/closure 分配 | `lib/service/transaction/transaction_mgr_impl.go`、`transaction_impl.go` | 每事务 channel + closure 闭包分配 | 待 profile |
| 3 | RabbitMQ frame 构建/publish 等待/backlog | `lib/service/bus/driver/rabbitmq/rabbitmq.go` | BuildFrame 拷贝、同步 publish 阻塞 | 待 profile |
| 4 | Gamedata 快照构建 | `module/gamedata/` | 热更全表重建的瞬时分配 | 待 profile |
| 5 | HTTP Client 连接复用 | `lib/api/http_client/client.go` | Transport 复用与 LimitReader 开销 | 待 profile |
| 6 | Sonic 在 Go 1.26 下回退 encoding/json 兼容 | 依赖 go.mod | 编解码路径 | 待 profile |

## 现有微基准（PRE-02 冻结，作为 before 基线）

| 包 | bench 文件 | 度量 |
|---|---|---|
| `lib/net/net_mgr` | `session_hub_bench_test.go` | SessionHub Lookup/Add/Remove |
| `lib/service/transaction` | `transaction_bench_test.go` | 事务提交/串行键派发 |

> 详细中位数见 [`micro-baseline.md`](./micro-baseline.md)（dev@4b595f4，WSL go1.26.5 ×10）。

## Profile 发现记录

> 每次取得 profile 后在此追加一节，格式：
> ```
> ### YYYY-MM-DD：<候选路径名>
> - 环境：WSL go1.26.5, GOMAXPROCS=N, 日志级别=release
> - profile 来源：C<n> 容量阶梯 / 微基准 cpu.prof
> - 热点：<top 函数与占比>
> - 优化：<最小改动描述>
> - benchstat：<关键指标 before→after，回退≤5%>
> - 结论：合并 / 回滚
> ```

（暂无 profile 证据，未修改任何候选文件。）
