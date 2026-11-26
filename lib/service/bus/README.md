# lib/service/bus

[功能] 定义服务间总线接口 `IBus`、地址解析和显式驱动注册表。只需抽象或自定义驱动时可独立使用。各消息队列驱动单独发布。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/bus@v0.1.0
```

## 示例

[运行条件] 解析地址不建立连接。实际使用时向 `NewDriverRegistry` 注册驱动，再调用 `CreateBus`、`Start` 和 `Close`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"log"
)

func main() {
	kind, _, err := bus.ParseAddr("nats://127.0.0.1:4222")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(kind, bus.IpStringToInt("1.2.3.4"))
}
```

---

# Bus 中间件抽象层

`bus` 是服务器之间消息收发的中间件抽象层，统一屏蔽不同 MQ 中间件（RabbitMQ / NSQ / NATS / Kafka / RocketMQ）的差异，对业务只暴露一套简单 API：

- `Send(dstBusId, data1, data2)`：发送到指定目标 busId（内部封装 `busPacketHeader`）
- `SetReceiver(onRecv)`：设置接收回调，**约定单协程顺序回调**，业务层无需再加锁

---

## 1. 使用方式总览

### 1.1 传统工厂（兼容旧代码）

```

此接口内部通过 `RegisterBus` 注册的构造器，找到对应实现并实例化；返回 `nil` 代表失败（并已写入日志）。

### 1.2 推荐：addr 一站式工厂（自动识别类型）

当前推荐风格：**只传一个 addr 字符串**，内部自动解析为不同 MQ 类型及其配置：

```go
b, err := bus.CreateBus(selfBusId, onRecv,
    "nats://127.0.0.1:4222?subject_prefix=bus&queue_group=g1")
if err != nil {
    // 创建失败：addr 格式错误 / 未注册实现 / 参数缺失等
}
```

`CreateBusE` 内部流程：

1. 调用 `ParseAddr(addr)`：
   - 解析出实现类型（implType）
   - 解析出对应的后端配置结构（RabbitMQConfig / NSQConfig / ...）
2. 根据 implType 从内部注册表中找到构造函数 `BusCtor`
3. 调用构造函数，返回一个 `IBus` 实例或 error

---

## 2. 支持的后端及 addr 格式

### 2.1 RabbitMQ

**方式一：直接传 AMQP URL（自动识别为 rabbitmq）**：

```go
b, err := bus.CreateBusE(selfBusId, onRecv,
    "amqp://guest:guest@127.0.0.1:5672/")
```

**方式二：显式 rabbitmq scheme：**

```go
b, err := bus.CreateBusE(selfBusId, onRecv,
    "rabbitmq://?addr=amqp://guest:guest@127.0.0.1:5672/")
```

内部会生成：

```go
type RabbitMQConfig struct {
    Addr string
}
```

并调用 `NewBusImplRabbitMQ(selfBusId, onRecv, conf.Addr)`。


### 2.2 NSQ

addr 示例：

```text
nsq://127.0.0.1:4150?lookup=127.0.0.1:4161&topics=bus&chan=ch&concurrency=3
```

对应内部配置：

```go
type NSQConfig struct {
    LookupAddrs []string
    NsqdAddr    string // host:port
    TopicPrefix string
    Channel     string
    Concurrency int
}
```

- `NsqdAddr` 来自 `nsq://` 的 host 部分（`127.0.0.1:4150`）
- `lookup` 参数支持多个地址，用逗号分隔
- `topics` 作为 `TopicPrefix`，默认 topic 命名为：`<TopicPrefix>_<busIdHex>`
- `chan` 作为 `Channel`
- `concurrency` 控制消费并发度

也可以直接手动构造 config 再调用实现构造函数：

```go
conf := bus.NSQConfig{
    LookupAddrs: []string{"127.0.0.1:4161"},
    NsqdAddr:    "127.0.0.1:4150",
    TopicPrefix: "bus",
    Channel:     "ch",
    Concurrency: 3,
}
busImpl := bus.NewBusImplNsqMQ(selfBusId, onRecv, conf)
```


### 2.3 NATS

addr 示例：

```text
nats://127.0.0.1:4222?subject_prefix=bus&queue_group=g1
```

对应内部配置：

```go
type NatsConfig struct {
    URL           string // nats://host:port
    SubjectPrefix string
    QueueGroup    string
}
```

- `URL` 由 `nats://` + host 组成
- `subject_prefix` 用作 subject 前缀：`<SubjectPrefix>.<bus_<hexBusId>>`
- `queue_group` 为 queue group（可选）


### 2.4 Kafka

addr 示例：

```text
kafka://127.0.0.1:9092,127.0.0.2:9092?topic_prefix=bus&group_id_prefix=g1
```

对应内部配置：

```go
type KafkaConfig struct {
    Brokers       []string
    TopicPrefix   string
    GroupIDPrefix string
}
```

- `Brokers` 按逗号分隔
- topic 命名：`<TopicPrefix>.<bus_<hexBusId>>`
- groupID 命名：`<GroupIDPrefix>.<bus_<hexBusId>>`


### 2.5 RocketMQ

addr 示例：

```text
rocketmq://127.0.0.1:9876?topic=goone_bus&consumer_group=goone_bus
```

对应内部配置：

```go
type RocketMQConfig struct {
    NameServers   []string
    Topic         string
    ConsumerGroup string
}
```

- `NameServers` 可为多个地址，逗号分隔
- `Topic` 为发送/订阅的 topic
- `ConsumerGroup` 为消费组名称

---

## 3. 统一协议：busPacketHeader

所有实现都遵循统一的包格式：

```text
packet = header(固定 12 bytes) + data1 + data2
```

- 发送侧：
  - 由各实现内部构造 `busPacketHeader{version, passCode, srcBusId, dstBusId}`
  - 按固定长度写入到缓冲区开头
  - 再依次拷贝 `data1` 与 `data2`

- 接收侧：
  - 从字节流解析出 `busPacketHeader`
  - 校验 `passCode` 是否匹配
  - 将剩余部分（`data1+data2` 拼接而成）作为 payload 传给：

```go
onRecv(srcBusId uint32, payload []byte) error
```

> 特别说明：`nsq` 已统一为与 `rabbitmq` / `rocketmq` 等相同的 header/payload 风格，老版本自定义 payload 风格请逐步迁移。

---

## 4. 并发模型与回调约定

- 每个 bus 实现内部会起若干 goroutine 负责：
  - 与 MQ 后端的长连接维护
  - 发送队列（`chanOut`）消费与投递
  - 接收队列（`chanIn`）与消息解析
- 但对业务只暴露一个约定：

> `onRecv` 回调保证在单协程中按顺序触发，不会并发调用。

因此**业务回调一般无需额外加锁**；如果自行在回调内起 goroutine，需要自行处理并发安全。

---

## 5. 调试与测试建议

- 推荐在本地起对应中间件（RabbitMQ / NSQ / NATS / Kafka / RocketMQ），配合 `go run ./tools/cmd/checkmodules test ./lib/service/bus` 进行联调；
- 如在 CI 环境无外部 MQ，可使用 `-run` / `-short` 或在测试中 `t.Skip`（当前测试已按不可用时自动跳过）；
- 建议统一通过 `CreateBusE` + addr 的方式管理配置，便于在配置中心统一下发/修改。
