# lib/net/tcp_server

[功能] 基于标准库的 TCP 监听和连接管理。`TcpSvr` 处理字节流，`TcpPacketSvr` 根据包头和长度解帧。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/net/tcp_server@v0.1.0
```

## 示例

[运行条件] 示例向系统申请本地端口并立即关闭。应用应在 `OnRead` 返回实际消费的字节数。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"github.com/Iori372552686/GoOne/lib/net/tcp_server"
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

func (h echo) OnRead2(conn net.Conn, data []byte) int { return h.OnRead(conn, data) }

func main() {
	server := &tcp_server.TcpSvr{}
	if err := server.InitAndRun("127.0.0.1", 0, echo{}); err != nil {
		log.Fatal(err)
	}
	if err := server.Stop(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```
