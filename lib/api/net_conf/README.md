# lib/api/net_conf

[功能] 根据 `NacosConf` 构造 Nacos v2 配置客户端。复用旧配置结构的服务可调用 `NewNacosConfigClient` 获取 SDK 客户端。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/net_conf@v0.1.0
```

## 示例

[运行条件] 运行需要本地 Nacos，命名空间和凭据按部署设置。退出时调用 `CloseClient`。希望使用统一配置源接口的项目可用 `lib/contrib/config/nacos`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"log"
	"os"
)

func main() {
	client, err := net_conf.NewNacosConfigClient(net_conf.NacosConf{
		IPAddr: "127.0.0.1", Port: 8848,
		NamespaceID: os.Getenv("NACOS_NAMESPACE"),
		UserName:    os.Getenv("NACOS_USERNAME"), Password: os.Getenv("NACOS_PASSWORD"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.CloseClient()
	fmt.Printf("%T\n", client)
}
```
