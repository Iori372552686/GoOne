# lib/service/router

[功能] 用实例发现与消息总线发送服务器间报文，可按 Bus ID、服务类型、UID 或 Router ID 寻址。入口包括 `New`、`InitAndRunWithBusCtor`、`ReadyCheck` 和 `Close`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/router@v0.1.0
```

## 示例

[运行条件] 示例只检查未启动路由器的状态。上线时显式提供总线构造器、注册中心地址和服务路由规则。关停先 `BeginShutdown`，再 `Close`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/router"
)

func main() {
	r := router.New()
	defer r.Close()
	fmt.Println(r.SelfBusId(), r.BusHealthy())
}
```
