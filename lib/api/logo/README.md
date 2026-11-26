# lib/api/logo

[功能] 提供服务进程的终端启动信息。使用 `Print` 显示服务名和内置框架版本。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/logo@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"github.com/Iori372552686/GoOne/lib/api/logo"
)

func main() {
	logo.Print("example-service")
}
```
