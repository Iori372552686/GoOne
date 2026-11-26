# lib/contrib/config/nacos

[功能] 用 Nacos v2 SDK 读取、订阅和发布配置。用 `New` 配置 group 与 Data ID，需要写入时调用 `NewPublisher`。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/nacos@v0.1.0
```

## 示例

[运行条件] 运行需要本地 Nacos 和已发布的 `demo.yaml`。先停止 watcher，再关闭 SDK 客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/nacos"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"log"
)

func main() {
	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &constant.ClientConfig{TimeoutMs: 5000, NotLoadCacheAtStart: true},
		ServerConfigs: []constant.ServerConfig{{IpAddr: "127.0.0.1", Port: 8848}},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.CloseClient()
	source, err := nacos.New(client, nacos.WithGroup("DEFAULT_GROUP"), nacos.WithDataIDs("demo.yaml"))
	if err != nil {
		log.Fatal(err)
	}
	items, err := source.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(items))
}
```
