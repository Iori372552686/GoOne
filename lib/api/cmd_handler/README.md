# lib/api/cmd_handler

[功能] 维护旧式 HTTP 和 WebSocket 处理器映射，供已有网关接线使用。`NewCmdHandlerMgr` 创建管理器，`HttpRegister` 与 `WsRegister` 注册回调。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/cmd_handler@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/gin-gonic/gin"
)

func main() {
	mgr := cmd_handler.NewCmdHandlerMgr()
	mgr.HttpRegister("health", func(_ *gin.Context, _ []byte) gin.H {
		return gin.H{"status": "ok"}
	})
	handler, found := mgr.GetHttpHandlers("health")
	if found {
		fmt.Println(handler(nil, nil))
	}
}
```
