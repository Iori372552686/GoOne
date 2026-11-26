# lib/contrib/registry/nacos

[功能] 用 Nacos v2 Naming SDK 注册、注销和发现服务，并支持集群和分组设置。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/nacos@v0.1.0
```

## 示例

[运行条件] 运行需要 Nacos。结束前停止 watcher、注销已注册实例，并关闭 Naming SDK 客户端。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/registry/nacos"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"log"
)

func main() {
	client, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &constant.ClientConfig{TimeoutMs: 5000, NotLoadCacheAtStart: true},
		ServerConfigs: []constant.ServerConfig{{IpAddr: "127.0.0.1", Port: 8848}},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.CloseClient()
	r, err := nacos.New(client, nacos.WithGroup("DEFAULT_GROUP"))
	if err != nil {
		log.Fatal(err)
	}
	instances, err := r.GetService(context.Background(), "inventory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(instances))
}
```

---

# Nacos Registry

## example
### server
```go
package main

import (
	"log"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"

	"github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/go-kratos/kratos/v2"
)

func main() {
	sc := []constant.ServerConfig{
		*constant.NewServerConfig("127.0.0.1", 8848),
	}
	

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ServerConfigs: sc,
		},
	)

	if err != nil {
		log.Panic(err)
	}

	r := nacos.New(client)

	// server
	app := kratos.New(
		kratos.Name("helloworld"),
		kratos.Registrar(r),
	)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```
### client
```go
package main

import (
	"context"
	"log"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"

	"github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

func main() {

	cc := constant.ClientConfig{
		NamespaceId: "public",
		TimeoutMs:   5000,
	}

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig: &cc,
		},
	)

	if err != nil {
		log.Panic(err)
	}

	r := nacos.New(client)

	// client
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///helloworld"),
		grpc.WithDiscovery(r),
	)
	defer conn.Close()
}
```
