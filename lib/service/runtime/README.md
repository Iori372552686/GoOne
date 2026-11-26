# lib/service/runtime

[功能] 统一管理应用配置、组件启动、就绪与退出。组件按注册顺序启动，退出时逆序停止，并可实现停止接入和排空接口。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/runtime@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"log"
	"time"
)

func main() {
	app := runtime.MustNew("example")
	app.MustRegister(runtime.ComponentFunc{
		ComponentName: "worker",
		OnStart: func(context.Context) error {
			fmt.Println("started")
			return nil
		},
		OnStop: func(context.Context) error {
			fmt.Println("stopped")
			return nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```
