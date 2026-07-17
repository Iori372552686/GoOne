# GoOne 可观测性指南

## 1. 指标（Prometheus）

每个服务的 Admin 端口（`base_cfg.runtime.admin_server`，默认 `8100+服务类型`）暴露 `/metrics`。

抓取配置示例：

```yaml
scrape_configs:
  - job_name: goone
    static_configs:
      - targets:
          - "10.0.0.1:8101"   # connsvr
          - "10.0.0.1:8102"   # mainsvr
          - "10.0.0.1:8103"   # infosvr
          - "10.0.0.1:8104"   # mysqlsvr
          - "10.0.0.1:8110"   # roomcentersvr（按实际类型端口）
```

### 指标清单


| 前缀                                                 | 关键指标                                                                                                                                          | 用途                                                                    |
| -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `goone_transaction_*`                              | `packets_total{phase,result,cmd}`、`handler_duration_seconds`、`active_transactions`、`pending_packets`、`dispatch_queue_length`、`timeouts_total` | 业务事务吞吐/延迟/积压/背压丢弃（含 `dropped_queue_full`、`dropped_deadline_exceeded`） |
| `goone_router_*`                                   | `messages_total{direction,transport}`、`message_bytes_total`、`message_errors_total`                                                            | 服务间消息流量与失败                                                            |
| `goone_gateway_*`                                  | `connections{transport,kind}`、`events_total{transport,event}`                                                                                 | 网关在线连接与生命周期事件                                                         |
| `goone_ssrpc_*`                                    | `requests_total`、`request_errors_total`、`request_timeouts_total`、`requests_in_flight`                                                         | RPC 层视角                                                               |
| `goone_redis_*` / `goone_mysql_*` / `goone_xorm_*` | 命令/操作速率、错误、超时、在途                                                                                                                              | 存储层健康                                                                 |
| `goone_safego_recovered_panics_total`              | —                                                                                                                                             | 业务 panic 捕获计数（非零应告警）                                                  |


### Grafana

导入 `[grafana-dashboard.json](grafana-dashboard.json)`（Grafana ≥ 10，数据源选择 Prometheus）。
面板分六组：Transaction / Router-Bus / Gateway / ssrpc / Storage / Runtime。

建议告警规则（示例阈值，按压测结果调整）：

- `increase(goone_safego_recovered_panics_total[5m]) > 0` — 业务 panic；
- `rate(goone_transaction_packets_total{result=~"dropped_.*"}[1m]) > 0` — 事务背压丢弃；
- `histogram_quantile(0.99, ...handler_duration...) > 0.5` — P99 处理超 500ms；
- `up == 0` 或 `/readyz` 非 200 — 实例失联 / bus 断连（readyz 已纳入 bus 健康）。

## 2. 全链路 Trace

自 SS 协议 v2（头长 86B）起，`TraceID(16B)/SpanID(8B)/DeadlineUnixMs(8B)` 在服务间透传：

- **root trace**：connsvr 转发客户端消息时生成（`router.SendMsgByConn`）；
- **透传**：`Transaction.Call`* 系列自动继承 TraceID、生成新 SpanID，并把
`min(剩余预算, 单跳 3s)` 写入下游 deadline（级联超时）；
- **接收端**：`TransactionMgr` 丢弃已超期请求（`dropped_deadline_exceeded` 指标）；
handler 内 `ctx.TraceID()`（ssrpc.Context）即可取到透传的 trace id，
Logging 中间件自动在日志前缀输出 `[trace_id:... span_id:...]`；
- **HTTP/gRPC 边界**：ssrpc 已支持 `traceparent` / `x-trace-id` 头的注入与提取。

> **协议兼容性警告**：SS 头 v2 是破坏性变更（86B vs 旧 54B），集群必须整组发版，
> 不支持新旧节点混布滚动升级。

### 采样与导出

`base_cfg.runtime.tracing` 控制 ssrpc trace 行为（`enabled/exporter/endpoint/sampler_ratio`）。
当 `exporter` 配置为 `otlphttp` 时，trace 经由 OTLP HTTP 协议导出到外部 collector；
其他取值（如内置轻量实现）走进程内日志通道。请确认 `endpoint` 指向可达的 collector。

## 3. 日志关联

- 事务日志前缀 `[uid|rid|transID]`，ssrpc 日志含 `trace_id`/`span_id`；
- 用 trace_id 在多服务日志中检索同一请求的完整路径；
- 心跳等高频 cmd 可用 `logger.RegisterCmdBacklist` 屏蔽。

