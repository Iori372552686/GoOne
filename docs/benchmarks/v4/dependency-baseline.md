# GoOne V4 起始依赖图基线

> 本文件冻结 V4 实施前的模块依赖图结论，作为 P0-09 CI 依赖图门禁的对照基线。
>
> 原始 `go list -deps` 与 `go mod graph` 输出存于本地忽略目录 `.artifacts/v4/`，不提交：
> - `websvr-deps.txt`（`go list -deps ./cmd/web_svr`，1205 行）
> - `connsvr-deps.txt`（`go list -deps ./cmd/connsvr`，1229 行）
> - `go-mod-graph.txt`（`go mod graph`，4258 行）

## 采集环境

| 项 | 值 |
|---|---|
| 提交 | `33bac24` |
| 命令 | `go list -deps ./cmd/web_svr` / `go list -deps ./cmd/connsvr` / `go mod graph` |

## 关键结论（V4 起始事实）

| 服务 | 是否引入 MQ SDK | 具体 MQ 依赖 |
|---|---|---|
| `cmd/web_svr` | **否**（无任何 MQ SDK） | — |
| `cmd/connsvr` | 是 | `github.com/rabbitmq/amqp091-go` + 本地 `lib/service/bus/driver/rabbitmq`（仅 RabbitMQ/amqp091） |

### 验证方式

对两个服务的 `go list -deps` 输出做大小写不敏感关键字匹配
（`amqp` / `rabbit` / `nats` / `kafka` / `stomp` / `sarama` / `pulsar` / `nsqio` / `rocketmq`）：

- websvr：**零命中** → 不链接任何 MQ SDK。
- connsvr：仅命中 `github.com/rabbitmq/amqp091-go` 与
  `github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq` → 唯一生产 MQ 驱动为
  RabbitMQ/amqp091。

## V4 期望的不变量（CI 门禁）

以下为 P0-09 与计划 §1.3 要求 CI 持续守护的依赖图不变量：

1. **websvr 不引入 MQ SDK**：web 服务是 HTTP/gRPC 入口，不参与 bus 事务循环，不得链接
   `amqp091-go` 或其他 MQ 客户端。
2. **connsvr 仅引入 RabbitMQ/amqp091**：网关是生产 MQ 主驱动的消费方，但不得同时引入
   NATS/Kafka/STOMP 等非生产驱动 SDK。
3. 非 RabbitMQ 驱动迁出根 module 属 P2-01 条件执行项；未完成前，根 module 不应因新代码
   引入新的 MQ SDK 依赖。

## 待办

- P0-09：将上述两条不变量固化为 CI job 的 `go list -deps` 检查步骤（与现有
  `.github/workflows/ci.yml` lint job 的 MQ 依赖图 gate 合并，不重复实现）。
