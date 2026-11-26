# lib/api/uerror

[功能] 维护旧式整数业务错误和消息。`New` 构造错误，`GetCodeMsg` 提取编码和文本。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/uerror@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
)

func main() {
	err := uerror.New(0, 40001, "invalid %s", "name")
	code, message := uerror.GetCodeMsg(err)
	fmt.Println(code, message)
}
```
