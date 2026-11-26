# lib/util/bufpool

[功能] 复用按容量管理的字节缓冲区。`Acquire` 和 `Release` 成对使用，`Get` 和 `Put` 是字节切片入口。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/bufpool@v0.1.0
```

## 示例

[运行条件] 归还缓冲区后，其 `Bytes` 不再由调用方持有。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
)

func main() {
	buf := bufpool.Acquire(4)
	defer bufpool.Release(buf)
	copy(buf.Bytes, "ping")
	fmt.Println(string(buf.Bytes))
}
```
