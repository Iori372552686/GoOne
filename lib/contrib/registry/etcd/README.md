# lib/contrib/registry/etcd

[功能] 使用 etcd 存储和发现服务实例，并支持 Watch。`New` 接收调用方创建的 etcd 客户端。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/etcd@v0.1.0
```

## 示例

[运行条件] 运行需要 etcd。若已注册实例，退出前用 `Deregister` 清理。先停止 watcher，再关闭 etcd 客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry/etcd"
	"go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

func main() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	r := etcd.New(client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	instances, err := r.GetService(ctx, "inventory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(instances))
}
```
