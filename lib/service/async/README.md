# lib/service/async

[功能] 提供异步任务队列、容量控制、拒绝新任务和排空。使用 `NewAsyncWithCapacity` 创建 worker，也可通过 `NewWorkerComponent` 接入生命周期。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/async@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/async"
	"log"
)

func main() {
	worker := async.NewAsyncWithCapacity(32)
	worker.Start()
	defer worker.Stop()
	done := make(chan struct{})
	if err := worker.PushE(func() {
		defer close(done)
		fmt.Println("task done")
	}); err != nil {
		log.Fatal(err)
	}
	<-done
}
```
