# lib/service/bus/driver/nats

[功能] 将 NATS 接入 GoOne 消息总线。通过 `Driver` 显式注册后创建 `bus.IBus`，由调用方管理连接生命周期。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/bus/driver/nats@v0.1.0
```

## 示例

[运行条件] 运行需要可访问的 NATS 服务。退出时调用 `Close`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/nats"
	"log"
	"time"
)

func main() {
	drivers := bus.NewDriverRegistry()
	drivers.MustRegister(nats.Driver())
	b, err := drivers.CreateBus(bus.IpStringToInt("1.2.3.4"), func(_ uint32, data []byte) error {
		fmt.Println(string(data))
		return nil
	}, "nats://127.0.0.1:4222?subject_prefix=bus&queue_group=demo")
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := b.Start(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println(b.Healthy())
}
```
