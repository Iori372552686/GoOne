# lib/db/redis

[功能] 按实例 ID 管理 Redis 连接及常用字节、Hash、计数和有序集合操作。支持 Standalone、Sentinel、Cluster 和 TLS。应用可用 `InitAndRun`，也可用 `NewComponent` 纳入服务生命周期。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/redis@v0.1.0
```

## 示例

[运行条件] 运行需要本地 Redis，示例键会在一分钟后过期。使用 `OnStart` 时通过 `NewRedisMgrWithConfigLoader` 注入应用自己的配置读取函数。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"log"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	manager := redis.NewRedisMgr()
	defer manager.Close()
	err := manager.InitAndRun(ctx, []redis.Config{{
		InstanceID: 1, Mode: string(redis.ModeStandalone),
		Addresses: []string{"127.0.0.1:6379"},
	}})
	if err != nil {
		log.Fatal(err)
	}
	if err := manager.SetBytes(ctx, 1, "goone:example", []byte("ok"), time.Minute); err != nil {
		log.Fatal(err)
	}
	value, err := manager.GetBytes(ctx, 1, "goone:example")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(value))
}
```
