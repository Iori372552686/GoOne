# lib/contrib/registry

[功能] 定义服务实例、注册、注销、发现与订阅接口。业务代码可接收 `Discovery`，装配层传入选定后端。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry@v0.1.0
```

## 示例

[运行条件] 持有 `Client` 的一方负责调用 `Close`，订阅结束时调用 `Watcher.Stop`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry"
)

func main() {
	instance := registry.ServiceInstance{
		ID: "inventory-1", Name: "inventory",
		Endpoints: []string{"grpc://127.0.0.1:9000"},
	}
	fmt.Println(instance.ID, instance.Endpoints[0])
}
```

---

# Registry 抽象层（Kratos 风格裁剪版）

本目录提供一套**服务注册 / 服务发现**的通用抽象，参考 Kratos registry 的接口形态，并在本项目里做了轻量封装与工程化落地。

## 目录结构

- `registry.go`
  - 仅包含**核心接口与数据结构**（不依赖任何后端中间件）
  - `Registrar`：注册 / 反注册
  - `Discovery`：获取服务 / Watch
  - `Watcher`：阻塞式 Next + Stop
  - `Client`：`Registrar + Discovery + Close()`
- `factory/`
  - **创建具体后端 Client 的工厂**：解析 `zk://...` / `consul://...` / `nacos://...` / `k8s://...` / `etcd://...`
  - 设计目标：让业务侧只传一个地址字符串即可完成选型与初始化
- `zookeeper/`、`consul/`、`nacos/`、`kubernetes/`、`etcd/`
  - 各中间件的具体实现（尽量保持与 `registry` 接口语义一致）

## 在 GoOne 中的落地点（router/svrinstmgr）

GoOne 的 `router.InitAndRun()` 内部会调用 `svrinstmgr.InitAndRun(selfBusID, routeRules, RegisterAddr)`（配置文件中推荐使用 `base_cfg.register_addr`）。

目前 `svrinstmgr` 已接入 `registry/factory`：

- 参数名使用 `RegisterAddr`：它实际是 **RegistryAddr**：既可以是 `host:port`（默认 zk），也可以是带 scheme 的 URL。
- 默认服务名：`online`
- 注册 key：`ID=selfBusID`，由 `selfBusID`（形如 `world.zone.type.ins`）解析出 `svrType` 用于路由分发。

## 配置格式（统一入口）

工厂入口：

- `factory.ParseConfig(addr string) (Config, error)`：只解析
- `factory.NewFromAddr(addr string) (registry.Client, Config, error)`：解析 + 创建 client

### 默认行为（无 scheme）

`"127.0.0.1:2181"`  
等价于 ZooKeeper 后端：**默认 backend=zk**，addr 作为 zk 地址。

### 支持的 scheme

#### ZooKeeper

`zk://127.0.0.1:2181?root=/&service=online&timeout=30s`

- `root`：zk 根路径（默认 `/`）
- `service`：服务名（默认 `online`）
- `timeout`：连接/操作超时（默认 `30s`）

#### Consul

`consul://127.0.0.1:8500?service=online&timeout=5s&healthcheck=true&heartbeat=true&health_interval=10`

- `healthcheck`：是否启用健康检查（默认 `true`）
- `heartbeat`：是否启用 TTL 心跳（默认 `true`）
- `health_interval`：健康检查间隔秒数（默认 `10`）

#### Nacos

`nacos://127.0.0.1:8848?service=online&timeout=5s&group=DEFAULT_GROUP&cluster=DEFAULT&kind=grpc&weight=100&nacos_namespace=public`

- `group` / `cluster` / `kind` / `weight`：对应 nacos registry option
- `nacos_namespace`：Nacos NamespaceId（可选）

#### Kubernetes

`k8s://?service=online&incluster=true`

或

`k8s://?service=online&kubeconfig=/path/to/kubeconfig`

说明：

- k8s 后端允许 **空 host**（因为连接信息来自 in-cluster 或 kubeconfig）
- 工厂会 `Start()` informer

#### etcd（可选启用）

`etcd://127.0.0.1:2379?service=online&timeout=5s&namespace=/microservices`

**注意：etcd 默认不启用，需要 build tag：**

`-tags registry_etcd`

原因见下文 “常见坑”。

## 常见坑 / 设计约束

### 1) etcd 的 protobuf extension 冲突（panic）

etcd 的依赖链里包含 `go.etcd.io/etcd/api/v3/versionpb`，它会在 init 阶段注册 `google.protobuf.FieldOptions` 扩展号（如 50001）。  
如果你的依赖树里还有其他库注册了同一个扩展号，会在启动时触发：

`panic: proto: extension number 50001 is already registered ...`

为避免“即使没用 etcd 也被 import 触发 init”，本项目将 etcd backend 做成可选：

- 默认：不链接 etcd（不会触发该类 panic）
- 需要 etcd：显式加 `-tags registry_etcd`

### 2) Watch 的 ctx 语义

约定：

- `Watch(ctx, name)` 返回的 watcher 应当在 `ctx` cancel 时尽快返回错误
- `Watcher.Stop()` 应当可重入/幂等（best-effort）

不同后端的底层能力不同，工厂在必要时会做 adapter（例如 k8s 的 Watch 形态）。

## 最小使用示例（伪代码）

```go
import (
  "context"
  "github.com/Iori372552686/GoOne/lib/contrib/registry/factory"
)

cli, cfg, err := factory.NewFromAddr("zk://127.0.0.1:2181?service=online&root=/")
_ = cfg
defer cli.Close()

_ = cli.Register(context.Background(), &registry.ServiceInstance{
  ID: "1.1.2.3",
  Name: "online",
})
```
