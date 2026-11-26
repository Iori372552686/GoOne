# lib/contrib/registry/consul

[功能] 基于 Consul agent 和健康服务列表执行服务注册与发现。`New` 接收 Consul SDK 客户端，可配置健康检查和名称解析。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/consul@v0.1.0
```

## 示例

[运行条件] 运行需要可访问的 Consul agent。有 ACL 时提供有权读取服务的 token。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry/consul"
	"github.com/hashicorp/consul/api"
	"log"
	"time"
)

func main() {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	r := consul.New(client)
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	instances, err := r.GetService(ctx, "inventory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(instances))
}
```
