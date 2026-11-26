# lib/util/timer

[功能] 用多级时间轮调度大量延时任务。使用 `Start`、`AddTimer`、`RemoveTimer` 和 `Stop` 管理时间轮。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/timer@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/timer"
	"time"
)

func main() {
	done := make(chan any, 1)
	wheel := timer.NewTimeWheel(func(value any) { done <- value })
	wheel.Start(context.Background())
	defer wheel.Stop()
	if wheel.AddTimer(100*time.Millisecond, "task-1", "ready") {
		fmt.Println(<-done)
	}
}
```
