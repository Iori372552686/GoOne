# lib/net/net_mgr

[功能] 为 TCP、WebSocket 和 KCP 提供共享会话、接入限流、按 UID 下行和会话切换。使用同一个 `SessionHub` 装配多个监听器。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/net/net_mgr@v0.1.0
```

## 示例

[运行条件] 示例只绑定内存连接。部署网关时加入实际的监听器、准入配置及 router，退出时先停止接入，再关闭监听器与会话。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"log"
	"net"
)

func main() {
	hub := net_mgr.NewSessionHub(nil)
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	client, _, err := hub.BindClientWithTag(local, 17, 1, "tcp")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.Uid, hub.GetClientByUid(17).Transport)
}
```
