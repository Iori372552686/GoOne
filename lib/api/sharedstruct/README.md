# lib/api/sharedstruct

[功能] 定义客户端和服务间包头、读取长度、命令字目标类型和 trace ID。适合要接入 GoOne 线协议的网关、客户端和服务。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/sharedstruct@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
)

func main() {
	var command uint32 = 0x020002
	fmt.Println(sharedstruct.ByteLenOfCSPacketHeader())
	fmt.Println(sharedstruct.ServerTypeInCmd(command))
	fmt.Printf("%x\n", sharedstruct.NewTraceID())
}
```
