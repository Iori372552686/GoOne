# GoOne 性能基线（baseline）

> 本文件是 GoOne 核心热路径的**冻结基线**，用于重构前后对比与回归门禁。
> 原始机器噪声（每次完整输出）放在忽略目录 `docs/benchmarks/raw/`，不提交。
> 配套：[architecture_review_2026-07-v2.md](../architecture_review_2026-07-v2.md)、[optimization_roadmap.md](../optimization_roadmap.md)。

## 采集约定

- 同一台机器、相同 Go 版本、相同参数，至少运行 10 次。
- 不同机器结果不可直接比较，仅与本机基线比较。
- 每次性能修改必须更新本文件，并附上前后两组数据。
- Transaction goroutine 模型是否改动，以 profile 证据为决策门禁，不凭直觉。

## 当前基线（2026-07-16 冻结）

| 项目 | 延迟 (ns/op) | 分配 (B/op) | allocs/op | 包 |
|---|---:|---:|---:|---|
| TransactionMgr throughput | 611–675 | 434–447 | 7–8 | `lib/service/transaction` |
| TransactionMgr serial key | 1499–1561 | 471–480 | 8 | `lib/service/transaction` |
| SSPacketHeader.ToBytes | 3.33–3.36 | 0 | 0 | `lib/api/sharedstruct` |
| SSPacketHeader.To | 2.41–2.43 | 0 | 0 | `lib/api/sharedstruct` |
| SSPacketHeader.From | 2.78–3.05 | 0 | 0 | `lib/api/sharedstruct` |
| CSPacketHeader.ToBytes | 11.28–11.90 | 32 | 1 | `lib/api/sharedstruct` |
| CSPacketHeader.From | 2.78–2.81 | 0 | 0 | `lib/api/sharedstruct` |
| bufpool.Get/Put | 18.46–19.16 | 24 | 1 | `lib/util/bufpool` |
| bufpool.Acquire/Release（Lease，0-alloc） | 7.46–7.78 | 0 | 0 | `lib/util/bufpool` |
| Scheduler TaskIdle（10ms 周期稳态） | 1000 | 0 | 0 | `lib/service/scheduler` |
| Scheduler TaskRun（单次执行） | 56.2–58.5 | 0 | 0 | `lib/service/scheduler` |
| Scheduler TaskNonOverlapSkip（TryLock 跳过） | 28.9–29.2 | 0 | 0 | `lib/service/scheduler` |
| Dispatcher DispatchWS (Sealed, 无锁只读 map) | 2.79–2.87 | 0 | 0 | `lib/service/ssrpc` |
| Dispatcher DispatchWS (Unsealed, RLock 旧路径) | 10.06–10.44 | 0 | 0 | `lib/service/ssrpc` |

### 采集环境

| 项 | 值 |
|---|---|
| 提交 | `4b58cf7c13ec097ca420ae884e598cc72ee77f98` (dev) |
| Go | `go1.25.10 windows/amd64` |
| CPU | `Intel(R) Core(TM) i7-14700KF` |
| GOMAXPROCS | 28 (runtime 默认) |
| CGO | disabled (race 测试在无 gcc 的本机不可执行，由 CI 承担) |
| 命令 | `go test -run '^$' -bench . -benchmem -count=10 ./lib/api/sharedstruct/... ./lib/service/transaction/... ./lib/util/bufpool/...` |

## 全量测试状态（冻结时）

- `go test -count=1 ./lib/... ./src/... ./common/... ./module/... ./tools/protoc-gen-goone/... ./tools/cmd/...` → **全部 PASS**。
- `go test -race` 本机因无 gcc 无法执行；CI (`.github/workflows/ci.yml` build-test job) 承担 `lib/net`、`lib/service/transaction`、`lib/service/router`、`lib/service/ssrpc`、`lib/util/safego` 的 race 检查。

## 回归门禁

- 任何“性能”提交必须给出同机前后 bench，且：
  - 核心吞吐中位数不回退超过 5%。
  - 0-alloc 热路径不得引入新分配。
- 后续 P1 会新增 Dispatcher lookup、Scheduler、Gateway encode/enqueue 等 benchmark，届时补入本表。

## P1 架构性验收（P0-08 达成，无 bench 数值，属行为级验收）

- **空闲服务无 application 100Hz Tick**：旧 `application.Run` 的 10ms 全局 Tick 已随 `lib/service/application` 包删除而消除；六个服务改用精确周期 Task，空闲时不再每秒被唤醒 100 次。
- **roomcenter 无每 10ms 双 goroutine**：原 `OnTick` 每 10ms 创建两个 goroutine（`RoomListMgr.Tick` + `TickPersist`）已替换为两个独立周期 Task（5s/10s，NonOverlap），不再每秒创建约 200 个短命 goroutine。
- **Dispatcher 热路径无注册锁**：见上表 `DispatchWS (Sealed)`，Seal 后走只读 map 无锁直查。
  - 0-alloc 热路径不得引入新分配。
- 后续 P1 会新增 Dispatcher lookup、Scheduler、Gateway encode/enqueue 等 benchmark，届时补入本表。
