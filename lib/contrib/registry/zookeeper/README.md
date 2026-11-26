# lib/contrib/registry/zookeeper

[功能] 通过 ZooKeeper 的临时节点跟踪服务实例和成员变化。使用 `New` 建立客户端，使用 `GetService` 查询实例。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/zookeeper@v0.1.0
```

## 示例

[运行条件] 运行需要可访问的 ZooKeeper。创建 watcher 后，用 `Stop` 停止监听，再关闭注册中心客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry/zookeeper"
	"log"
	"time"
)

func main() {
	r, err := zookeeper.New([]string{"127.0.0.1:2181"})
	if err != nil {
		log.Fatal(err)
	}
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
