# GoOne V4 起始微基准基线（×10）

> 本文件是 V4 现代化实施前的微基准冻结基线，供 P1-04 证据驱动性能优化前后做
> `benchstat` 对比。
>
> **重要：** 本批数据采集自 **Windows 开发机（go1.25.10 windows/amd64）**，仅用于开发
> 回归。性能门禁必须以固定 Linux 机器、固定 Go 版本与固定 GOMAXPROCS 的同机对比为准
> （计划 PRE-01 / P1-04）。Windows 与 Linux 结果**不可直接比较**。

## 采集环境

| 项 | 值 |
|---|---|
| 提交 | `33bac24`（紧随 V4 基线 `4b595f4`，PRE-01 文档提交之后） |
| 平台 | `windows/amd64`（开发机，仅用于开发回归） |
| Go | `go1.25.10` |
| GOMAXPROCS | 28（runtime 默认，`-28` 后缀） |
| 日志级别 | 默认（含 Info，未关闭——开发回归设定） |
| 命令 | `go test -run '^$' -bench . -benchmem -count=10 ./lib/api/sharedstruct/... ./lib/service/transaction/... ./lib/service/ssrpc/... ./lib/net/...` |

## 微基准中位数（×10）

| 项目 | ns/op（中位数） | 区间 | B/op | allocs/op | 包 |
|---|---:|---:|---:|---:|---|
| SSPacketHeader.To | 2.43 | 2.42–2.45 | 0 | 0 | `lib/api/sharedstruct` |
| SSPacketHeader.From | 2.99 | 2.98–3.02 | 0 | 0 | `lib/api/sharedstruct` |
| SSPacketHeader.ToBytes | 3.38 | 3.34–3.44 | 0 | 0 | `lib/api/sharedstruct` |
| CSPacketHeader.From | 2.86 | 2.79–3.27 | 0 | 0 | `lib/api/sharedstruct` |
| CSPacketHeader.ToBytes | 11.63 | 11.47–11.83 | 32 | 1 | `lib/api/sharedstruct` |
| DispatchWS (Sealed) | 2.79 | 2.77–2.84 | 0 | 0 | `lib/service/ssrpc` |
| DispatchWS (Unsealed) | 10.11 | 10.05–10.42 | 0 | 0 | `lib/service/ssrpc` |
| DispatcherCMD (Sealed) | 5.15 | 4.69–5.71 | 0 | 0 | `lib/service/ssrpc` |
| Registry Seal 100 bindings | 22700 | 22298–23210 | 33424 | 435 | `lib/service/ssrpc` |
| TransactionMgr throughput | 647.10 | 613.30–685.70 | 424–447 | 7–8 | `lib/service/transaction` |
| TransactionMgr serial key | 1527.50 | 1491.00–1628.00 | 471–480 | 8 | `lib/service/transaction` |
| SessionHubLookup | 11.07 | 11.02–11.14 | 0 | 0 | `lib/net/net_mgr` |
| SessionHubBindReplace | 1121.00 | 1080.00–1582.00 | 1879 | 17 | `lib/net/net_mgr` |

## 与 V3 基线的一致性

数值与本目录 `../baseline.md` 的 V3 节（同开发机 go1.25.10）一致，确认 V4 起始点与 V3
收敛点无回归：

- 0-alloc 热路径（SS `To`/`From`/`ToBytes`、CS `From`、DispatchWS Sealed、SessionHubLookup）
  在 V4 起始仍为 0 分配——这是 P1-04 不得破坏的不变量。
- CS `ToBytes` 维持 1-alloc（兼容接口，已知非零分配路径）。
- TransactionMgr throughput 中位数 ~647，与 V3 的 600–680 区间吻合。

## 待办（P1-04 触发时）

- 在固定 Linux 环境关闭 Info 日志后重新采集 ×10，作为正式性能门禁基线（`before.txt`）。
- 每个性能提交最多选择一个候选路径；profile 未显示为热点的候选文件不得修改。
- 合并门禁：同机器、同 Go、同 GOMAXPROCS、同日志级别；中位数回退不超过 5%；0-alloc 路径
  不得新增分配。
