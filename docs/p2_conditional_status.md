# P2 条件执行项状态

> V4 计划 §6：P2 全部为**条件执行项**——每项都有明确的激活门槛，未满足时标记
> **Deferred**，不得以「版本更新」「看起来更好」为由提前执行。
>
> 本文档固化截至 2026-07-31 的前置条件核对证据与判定，使结论可审计。
> 门槛满足后，在对应小节追加「激活记录」（分支、证据、回滚提交）。

## 汇总判定

| 项 | 判定 | 阻塞原因 |
|---|---|---|
| P2-01 基础依赖代际 | **Deferred** | 每项需独立分支 + 完整证据矩阵（build/unit/integration/race/断线恢复/benchmark），当前无对应分支与隔离验证环境 |
| P2-02 gnet v2 Linux A/B | **Deferred** | 需独立 Linux 分支，且吞吐≥+10% 或 CPU/RSS≥-15% 门槛未验证 |
| P2-03 PGO | **Deferred** | 需 C3/C4 或生产 CPU profile，且容量稳定提升≥5% 未取得 |
| P2-04 Agones Adapter | **Deferred** | 当前部署为 VM/Ansible，未进入 Kubernetes/Agones |
| P2-05 下一主版本破坏性清理 | **Deferred** | 需切换主版本；Deprecated 提示已在 P0/P1 预埋（见下） |

---

## P2-01 基础依赖代际

每项独立分支、独立提交、独立回滚。**前置证据要求**：API/协议兼容矩阵、
build+unit+integration+race、断线恢复或数据一致性测试、benchmark/容量前后对比、
单独回滚提交。

| 子项 | 当前版本（go.mod 证据） | 状态 |
|---|---|---|
| 1. Sonic | `github.com/bytedance/sonic v1.14.2` | Deferred：需确认支持目标 Go 版本或从非必要热路径移除，配合 P1-04 profile |
| 2. protobuf 迁移 | `golang/protobuf v1.5.4` 与 `google.golang.org/protobuf v1.36.11` 并存 | Deferred：迁移面大（生成代码 + 业务），需独立分支与兼容矩阵 |
| 3. Nacos v1/v2 统一 | `nacos-sdk-go v1.1.6`（旧）+ `nacos-sdk-go/v2 v2.3.5` 并存 | Deferred：需确认 v1 调用方全部迁出 |
| 4. XORM | `go-xorm/xorm v0.7.9`（老代，`xorm.io/builder`/`core` 为 indirect） | Deferred：需评估新版或替代方案 + ORM 兼容 |
| 5. Redis 客户端 | 见 go.mod redis 客户端版本 | Deferred：需评估 context 支持、连接池维护状态 |
| 6. 非 RabbitMQ 驱动迁出根 module | kafka/nats/nsq/rocketmq 仍在 driver 目录 | Deferred：需确认无生产链接后迁出 |

**激活条件**：任一子项在独立分支补齐证据矩阵后，合并并追加本节记录。

---

## P2-02 gnet v2 Linux A/B

- 当前：`github.com/panjf2000/gnet v1.6.7`（v1）。
- **迁移门槛**：吞吐 ≥ +10%，或 CPU/RSS ≥ -15%；且 C4 + race + 断线恢复 +
  Quiesce/Drain/Stop 全通过，TCP/WS/KCP 统一 SessionHub 契约兼容。
- **判定**：Deferred。需在独立 Linux 分支实施 A/B，未达门槛保留 v1，不强制迁移。

---

## P2-03 PGO

- **前置**：使用代表性 C3/C4 或生产 CPU profile；无 profile 时能回退；
  profile 不含敏感业务数据。
- **门槛**：容量矩阵稳定提升 ≥ 5%，且 P99/P999 不恶化。
- **判定**：Deferred。依赖 P1-03 C3/C4 阶梯或生产 profile，当前无。

---

## P2-04 Agones Adapter

- 当前部署：VM/Ansible（`deploy/`：`playbook_dev`、`roles`、`inithost`、
  `install.sh`、`deploy.sh`），无 Kubernetes/Agones。
- **计划明确**：「当前 VM/Ansible 部署不引入 Agones SDK」。
- **判定**：Deferred。确认生产进入 K8s/Agones 后再实现 Ready/Health/Reserve/
  Allocate/Shutdown + WatchGameServer + SIGTERM Drain cause + 驱逐/滚动/中断测试。

---

## P2-05 下一主版本破坏性清理

计划要求删除一批兼容路径，**前置条件**：「必须提供迁移文档和至少一个稳定版本的
Deprecated 提示」。截至 2026-07-31，Deprecated 提示已在 P0/P1 阶段预埋：

| 待删项 | Deprecated 提示状态 | 来源 |
|---|---|---|
| `SetHub` 与 hub 可选分支 | ✅ 已标 Deprecated | P0-04（`lib/net/net_mgr/net_i.go`） |
| 旧 `LoadXConfig` 全局发布接口 | 待核 | — |
| appconfig 通用 Reload Store | 待核 | — |
| 包级 Driver Registry + driver `init()` 兼容路径 | ✅ 已限制（生产强制 DriverRegistry，静态门禁） | P0-09 CI 扫描 |
| 非 RabbitMQ 驱动根模块依赖 | ✅ 已限制（依赖图门禁） | P0-09 CI 扫描 |
| 无错误返回的 BusID/IP 解析接口 | 待核 | — |
| `Client.Uid/Ip` 旧拼写 | 待核（P1-06 记录） | — |
| `lib/web/http_client` 代理包 | ✅ 已标 Deprecated | P0-08 |
| 失真的 `tools/tester/cmd/capacity` | ✅ 已标 Deprecated | P1-03 |
| `lib/util/file.MatchRemoveAll` | 待核 | — |
| `net_conf.NewNacosConfigClient` | ✅ 已标 Deprecated | P0-06 |

**判定**：Deferred。核心兼容路径的 Deprecated 提示已预埋；实际删除需切换主版本，
并提供迁移文档与至少一个稳定版本的过渡期。切换主版本时按上表逐项核销「待核」项。

---

## 激活记录模板

门槛满足后，在对应小节追加：

```
### YYYY-MM-DD：P2-0x 激活
- 分支：<branch>
- 证据：<build、unit、integration、race、benchmark 链接或摘要>
- 门槛达成：<量化指标>
- 回滚提交：<commit>
- 结论：合并 / 回滚
```
