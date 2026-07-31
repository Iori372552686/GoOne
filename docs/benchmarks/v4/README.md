# GoOne V4 现代化基线证据目录

> 本目录冻结 V4 现代化实施计划（`docs/modernization_execution_plan_2026-07-v4.md`）
> 的起始基线，并承载 P0/P1 期间的 benchmark、profile 与容量证据摘要。
>
> **配套文档：**
> - 计划：[`../../modernization_execution_plan_2026-07-v4.md`](../../modernization_execution_plan_2026-07-v4.md)
> - V3 基线：[`../baseline.md`](../baseline.md)

## 目录约定

| 文件 | 用途 |
|---|---|
| `README.md` | 本文件，记录基线 commit、环境与冻结时的门禁快照 |
| `micro-baseline.md` | V4 起始微基准（×10）摘要，供优化前后 benchstat 对比 |
| `dependency-baseline.md` | websvr/connsvr 依赖图结论与 go mod graph 来源 |
| `profile-findings.md`（P1-04 创建） | 证据驱动性能优化的 profile 结论 |
| `capacity-matrix.md`（P1-03 创建） | C1～C4 容量阶梯矩阵 |

原始噪声（每次完整 benchmark/profile 输出、容量 JSON）放入本地忽略目录
`.artifacts/v4/`，**不提交**；仓库只提交摘要、benchstat 与容量矩阵。

## 采集约定

- 同一台机器、相同 Go 版本、相同 GOMAXPROCS、相同日志级别。
- **Windows 结果仅用于开发回归；Linux 结果才可用于性能门禁**（见计划 PRE-01）。
- 每次性能修改必须给出同机前后 bench，核心吞吐中位数回退不超过 5%，0-alloc 热路径不得
  引入新分配。

## 起始基线（PRE-01，2026-07-31 冻结）

### 提交与环境

| 项 | 值 |
|---|---|
| 分支 | `dev` |
| 提交 | `4b595f4`（`4b595f479da6c0c88a98920aca25d78cc936cb83`） |
| 工作树 | 干净（仅本计划文档为未跟踪新增） |
| 平台 | `windows/amd64`（开发机，仅用于开发回归） |
| Go | `go1.25.10` |
| `go.mod` 基线 | `go 1.25.4`，**无 toolchain 指令**（P0-01 将增加 `toolchain go1.25.12`） |
| GOMAXPROCS | 运行时默认 |
| CGO_ENABLED | `0`（race 检测在本机无 gcc 不可执行，由 CI 承担） |
| CPU | Intel(R) Core(TM) i7-14700KF |

### 冻结时门禁快照

| 门禁 | 命令 | 结果 |
|---|---|---|
| Build | `go build ./...` | ✅ 通过 |
| Vet | `go vet -composites=false ./...` | ✅ 通过 |
| Unit | `go test -count=1 -timeout 600s ./...` | ⚠️ 60 包 PASS / 0 包 FAIL，1 个**先于 V4 的偶发**失败（见下） |

### 已知先于 V4 的偶发失败（非本次引入，不在 V4 范围内）

- `lib/util/random` 的 `TestLimit`（`rand_test.go:55-58`）：`Intn(100)` 返回 `[0,100)`，
  断言却要求 `ret > 0`，因此当取到 `0`（约 1% 概率）时失败。5 次单独重跑均 PASS，属 flaky。
  - 依据"精确修改"原则，V4 任务不触碰该测试；仅在此登记为既有 flaky，不与 V4 验收混淆。

### 待补充（PRE-02）

- 微基准 ×10 摘要（`micro-baseline.md`）。
- 依赖图结论：websvr 不含 MQ SDK、connsvr 仅含 RabbitMQ/amqp091（`dependency-baseline.md`）。
