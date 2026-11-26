# lib/util/safego

[功能] 封装带 panic 记录的 goroutine，并提供 `SingleFlight` 合并同键的并发操作。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/safego@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/safego"
)

func main() {
	done := make(chan struct{})
	safego.Go(func() {
		defer close(done)
		fmt.Println("work done")
	})
	<-done
}
```
