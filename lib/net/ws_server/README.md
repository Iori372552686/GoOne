# lib/net/ws_server

[功能] 提供基于 Gin 和 Gorilla WebSocket 的二进制接入、超时与连接关闭。入口是 `WsTcpSvr.InitAndRun`、`WriteData` 和 `Stop`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/net/ws_server@v0.1.0
```

## 示例

[运行条件] 示例打开一个临时监听器并立即关闭。应用在回调中处理或复制报文，并在退出时调用 `Stop`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"github.com/Iori372552686/GoOne/lib/net/ws_server"
	"log"
	"net"
)

type handler struct{}

func (handler) OnConn(net.Conn)                    {}
func (handler) OnClose(net.Conn)                   {}
func (handler) OnRead(_ net.Conn, data []byte) int { return len(data) }

func main() {
	server := &ws_server.WsTcpSvr{}
	if err := server.InitAndRun("", "release", 0, handler{}); err != nil {
		log.Fatal(err)
	}
	if err := server.Stop(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```
