# 容量矩阵（C1～C4）

> V4 P1-03：GoOne 容量测试的唯一主路径是 **stress** 工具
> （`tools/tester/cmd/stress`），它使用真实 Session、关联登录与业务响应码、采集
> 客户端 pprof，并输出 `stress_*.json` 原始报告。`tools/tester/cmd/capacity` 已
> **Deprecated**（把 socket Write 成功误认为登录/心跳成功，不校验响应码，不作容量证据）。
>
> 本文档由 `report.CapacityMatrix` 汇总多份 `stress_*.json` 自动生成；下方为骨架，
> 完成 C1～C4 阶梯后用工具刷新。

## 阶梯定义

| 阶段 | 长连接 | 登录速率 | 稳态消息 | 稳态时间 |
|---|---:|---:|---:|---:|
| C1 | 1,000 | 100/s | 500/s | 30 分钟 |
| C2 | 3,000 | 200/s | 1,500/s | 30 分钟 |
| C3 | 5,000 | 300/s | 2,500/s | 30 分钟 |
| C4 | 10,000 | 500/s | 5,000/s | 30 分钟 |

每档必须通过后才能进入下一档；失败先保存 profile 和原始 JSON，不直接修改代码。

## 容量矩阵（自动汇总）

> 生成方式：
> ```go
> m, _ := report.ParseMatrix("./.artifacts/stress")
> _ = os.WriteFile("docs/benchmarks/capacity-matrix.md",
>     []byte(report.RenderMarkdown(m)), 0o644)
> ```

| 阶段 | 目标连接 | 峰值在线 | 总请求 | 错误数 | 成功率 | P99 | SLO |
|---|---:|---:|---:|---:|---:|---:|:---:|
| C1 | — | — | — | — | — | — | 待执行 |
| C2 | — | — | — | — | — | — | 待执行 |
| C3 | — | — | — | — | — | — | 待执行 |
| C4 | — | — | — | — | — | — | 待执行 |

## C4 验收标准

- 连接成功率不低于 99.9%。
- 登录与业务请求成功率不低于 99.9%。
- 框架链路 P99 不高于 50ms。
- CPU 不高于分配核心的 70%。
- 最后 15 分钟 RSS 增长小于 5%。
- GC pause P99 不高于 20ms。
- goroutine、FD 无持续增长。
- readiness 在 1 秒内关闭。
- 已接受请求不丢失。
- 30 秒 drain timeout 内完成排空。
- Drain 后 goroutine、FD 回到基线 ±2%。

> CPU/RSS/GC/readiness/drain 指标由服务端 Prometheus 采集，判定标准见
> [`observability_slo.md`](../observability_slo.md)。

## 排水（Drain）记录

每档记录：readiness 关闭时间、已接受请求完成数、强制 Stop 数、资源恢复时间。
（待 C1～C4 执行后填充。）
