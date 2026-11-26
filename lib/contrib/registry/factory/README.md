# lib/contrib/registry/factory

[功能] 解析注册中心地址并构造统一客户端。后端有 ZooKeeper、etcd、Consul、Kubernetes 和 Nacos。只需要单个后端时可直接导入对应模块。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/factory@v0.1.0
```

## 示例

[运行条件] 运行示例需要本地 ZooKeeper。服务实例退出时先注销，订阅停止后再关闭客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry/factory"
	"log"
	"time"
)

func main() {
	client, cfg, err := factory.NewFromAddr("zk://127.0.0.1:2181")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	instances, err := client.GetService(ctx, "inventory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(cfg.Backend, len(instances))
}
```
