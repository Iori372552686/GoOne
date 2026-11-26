# lib/contrib/config/etcd

[功能] 从 etcd 读取或监听指定键、前缀，并可用 `NewPublisher` 写入配置。直接接入时用 `New` 和现有 etcd 客户端。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/etcd@v0.1.0
```

## 示例

[运行条件] 运行需要可访问的 etcd 和相应键权限。结束前停止 watcher，再关闭客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/etcd"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	source, err := etcd.New(client, etcd.WithContext(ctx), etcd.WithPath("/apps/demo"), etcd.WithPrefix(true))
	if err != nil {
		log.Fatal(err)
	}
	items, err := source.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(items))
}
```

---

# Etcd Config

```go
import (
	cfg "github.com/Iori372552686/GoOne/lib/contrib/config/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

// create a etcd client
client, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: time.Second,
    DialOptions: []grpc.DialOption{grpc.WithBlock()},
})
if err != nil {
    log.Fatal(err)
}

// configure the source, "path" is required
source, err := cfg.New(client, cfg.WithPath("/app-config"), cfg.WithPrefix(true))
if err != nil {
    log.Fatalln(err)
}

// load kvs
kvs, err := source.Load()
_ = kvs
_ = err

```
