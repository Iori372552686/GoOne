# lib/util/tos

[功能] 提供进程参数、主机信息、目录遍历和本地端口辅助函数。`GetFreePort` 返回已经占有端口的监听器。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/tos@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/tos"
	"log"
)

func main() {
	listener, err := tos.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	fmt.Println(listener.Addr())
}
```
