# lib/service/scheduler

[功能] 提供可纳入生命周期的周期任务，支持运行模式、跳过统计和关闭等待。入口为 `New`、`Start`、`Stop`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/scheduler@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"log"
	"time"
)

func main() {
	ctx := context.Background()
	done := make(chan struct{}, 1)
	task := scheduler.New("example", 10*time.Millisecond, func(context.Context) error {
		select {
		case done <- struct{}{}:
		default:
		}
		return nil
	})
	if err := task.Start(ctx); err != nil {
		log.Fatal(err)
	}
	<-done
	if err := task.Stop(ctx); err != nil {
		log.Fatal(err)
	}
}
```
