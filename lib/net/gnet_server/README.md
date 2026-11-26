# lib/net/gnet_server

[功能] 提供基于 gnet 的事件驱动 TCP 和 UDP 服务。TCP 用 `NewTcpServer` 注入包头长度、包体长度解析和事件处理器。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/net/gnet_server@v0.1.0
```

## 示例

[运行条件] 示例使用空闲的本地 TCP 端口。回调在事件循环内同步执行，退出时调用 `Stop`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"encoding/binary"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/net/gnet_server"
	"log"
	"net"
)

type handler struct{}

func (handler) OnConn(net.Conn)  {}
func (handler) OnClose(net.Conn) {}
func (handler) OnPacket(_ net.Conn, data []byte) {
	fmt.Println(len(data))
}

func main() {
	server := gnet_svr.NewTcpServer(2, func(header []byte) int {
		return int(binary.BigEndian.Uint16(header))
	}, handler{})
	if err := server.Start("127.0.0.1", 18081); err != nil {
		log.Fatal(err)
	}
	if err := server.Stop(); err != nil {
		log.Fatal(err)
	}
}
```
