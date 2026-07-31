# 集成适配器支持矩阵

> V4 P1-07：明确每个可选适配器（配置中心、服务注册发现、消息总线）的支持等级与
> 测试覆盖，避免「能编译即误以为生产可用」。
>
> **分级规则（仅三档，不得新增）：**
> - **Production**：进入 CI build + race + 真实 integration + 故障恢复矩阵。任一
>   外部依赖断开不杀死进程，watcher 可取消并 join，构造失败返回 error。
> - **Experimental**：可编译且有基础单测，但不承诺生产恢复语义（断线/重连/重试
>   行为未在 CI 验证）。可用于评估与沙箱。
> - **Compile-only**：仅保证根模块 `go build ./...` 通过，无测试覆盖；下一主版本
>   迁出或删除。
>
> **未进入 CI 真实集成矩阵的适配器不得标记 Production。**

## 配置中心（lib/contrib/config）

| 适配器 | 等级 | 构造期失败 | 运行期失败 | CI 真实集成 | 备注 |
|---|---|---|---|---|---|
| **nacos** | **Production** | `factory.NewClient` 返回 error | `Load`/`Watch` error 经 `InitRemote` 严格模式回滚；`StopNet` 取消监听 | 是（P0-06 gamedata 路径） | 生产配置中心主路径，gamedata 远端加载唯一后端 |
| **etcd** | **Experimental** | `factory.NewClient` 返回 error | watcher `Next` 返回 `context.Canceled` | 否（需 `-tags config_etcd`） | 有单测；build tag 保护，未在 CI 真实集成矩阵 |
| **consul** | **Experimental** | `factory.NewClient` 返回 error | watch-plan error 路由到 `Next() error`，不 panic（P1-07 修复） | 否 | 有单测；consul 服务端未进 CI |
| **apollo** | **Experimental** | `NewSourceE` 返回 error；`MustNewSource` panic；`NewSource` Deprecated（P1-07） | agollo 内部异步重连 | 否 | 有单测；apollo 服务端未进 CI |
| **kubernetes** | **Experimental** | 构造不 panic | informer list 失败记录跳过不 panic（P1-07 修复） | 否 | 无生产 K8s 集群验证 |

## 服务注册发现（lib/contrib/registry）

| 适配器 | 等级 | 构造期失败 | 运行期失败 | CI 真实集成 | 备注 |
|---|---|---|---|---|---|
| **etcd** | **Production** | `NewFromAddr` 返回 error | watcher 可取消 | 是（`svrinstmgr` + GOONE_ETCD_ADDR） | 注册发现默认后端 |
| **zookeeper** | **Production** | `NewFromAddr` 返回 error | 注册为临时节点，断线自动清理 | 是（`svrinstmgr` + zk service） | 历史默认后端，与 etcd 同为 Production |
| **nacos** | **Experimental** | `NewFromAddr` 返回 error | nacos naming 重连 | 否 | 有单测；未进 CI 真实注册矩阵 |
| **consul** | **Experimental** | `NewFromAddr` 返回 error | agent 注册 + watch | 否 | 有单测 |
| **kubernetes** | **Compile-only** | 构造不 panic | informer list 失败记录跳过不 panic（P1-07 修复） | 否 | 仅保证构建；无 K8s 集群验证 |

## 消息总线（lib/service/bus/driver）

| 适配器 | 等级 | `Start` 契约 | `Send` 失败语义 | CI 真实集成 | 备注 |
|---|---|---|---|---|---|
| **rabbitmq** | **Production** | 同步首连（Dial→Queue→Consume） | 同步 publish，error 回传（P0-07） | 是（`GOONE_AMQP_ADDR`） | 生产 MQ 唯一后端；connsvr/mainsvr 链接 |
| **kafka / nats / nsq / rocketmq** | **Experimental** | `Start` 轮询 `Healthy` 至 ctx 超时 | 进本地 chan，不回传 publish error | 否 | `Start` 已补齐（P0-07），但未进真实集成矩阵；生产链路不链接 |

## 故障恢复不变量（仅 Production 适配器承诺）

1. 外部服务断开**不会杀死进程**（无库 goroutine panic）。
2. watcher / consumer / publisher **可取消并 join**（`Stop` 幂等，goroutine 不泄漏）。
3. 构造失败**返回 error**，调用方可回滚，不留半启动状态。
4. 运行期断线经 `RuntimeErrors` / `Next() error` / 退避重连上报，触发上层 Drain/Failed。

Experimental / Compile-only 适配器不承诺上述不变量中的恢复语义，仅保证「不 panic 杀进程」（P1-07 已统一修复 consul/k8s/apollo）。
