# lib/contrib/config/factory

[功能] 解析配置中心地址并构造 `config.Client`。支持 Consul、Apollo、Nacos、Kubernetes 和 etcd，配置中心 etcd 工厂需要 `config_etcd` 构建标签。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/factory@v0.1.0
```

## 示例

[运行条件] 运行示例需要本地 Consul KV。只接一个后端的项目可以直接调用该后端模块。借用的 watcher 使用完应先停止。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/factory"
	"log"
)

func main() {
	client, cfg, err := factory.NewFromAddr("consul://127.0.0.1:8500?path=apps/demo")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	items, err := client.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cfg.Backend, len(items))
}
```
