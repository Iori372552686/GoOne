# lib/util/mutex

[功能] 提供可尝试加锁的互斥锁以及递归互斥锁。入口包括 `Mutex.TryLock`、`RecursiveMutex` 和 `TokenRecursiveMutex`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/mutex@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/mutex"
)

func main() {
	var locked mutex.Mutex
	if locked.TryLock() {
		defer locked.Unlock()
		fmt.Println("acquired")
	}
}
```
