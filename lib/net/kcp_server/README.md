# lib/net/kcp_server

[功能] 基于 UDP 提供 KCP 流监听与自定义包帧切分。`KcpSvr` 处理字节流，`KcpPacketSvr` 提供解帧回调。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/net/kcp_server@v0.1.0
```

## 示例

[运行条件] 服务通过 UDP 端口提供 KCP 接入。关闭时调用 `Stop`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"github.com/Iori372552686/GoOne/lib/net/kcp_server"
	"log"
	"net"
)

type echo struct{}

func (echo) OnConn(net.Conn)  {}
func (echo) OnClose(net.Conn) {}
func (echo) OnRead(conn net.Conn, data []byte) int {
	n, _ := conn.Write(data)
	return n
}

func main() {
	server := &kcp_server.KcpSvr{}
	if err := server.InitAndRun("127.0.0.1", 0, echo{}); err != nil {
		log.Fatal(err)
	}
	if err := server.Stop(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```
