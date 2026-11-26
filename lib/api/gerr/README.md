# lib/api/gerr

[功能] 构造和提取 GoOne 协议错误码与原因。适用于需要保留机器可读错误码、包装原始错误的处理器。入口有 `New`、`Wrap`、`Code` 和 `Reason`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/gerr@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	protocol "github.com/Iori372552686/g1_common/protocol"
)

func main() {
	err := gerr.New(protocol.ErrorCode_ERR_ARGV, "invalid_player", "玩家 ID 无效")
	fmt.Println(gerr.Code(err), gerr.Reason(err))
}
```
