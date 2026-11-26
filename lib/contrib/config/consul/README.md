# lib/contrib/config/consul

[功能] 从 Consul KV 路径读配置并监听变化。入口为 `New`、`WithPath`、`Load` 和 `Watch`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/consul@v0.1.0
```

## 示例

[运行条件] 运行前准备可访问的 Consul agent、读取 KV 的凭据和 `apps/demo` 前缀数据。用 `Watcher.Stop` 停止订阅。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/consul"
	"github.com/hashicorp/consul/api"
	"log"
)

func main() {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	source, err := consul.New(client, consul.WithPath("apps/demo"))
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

---

# Consul Config

```go
import (
    "github.com/go-kratos/kratos/contrib/config/consul/v2"
    "github.com/hashicorp/consul/api"
)
func main() {

    consulClient, err := api.NewClient(&api.Config{
    Address: "127.0.0.1:8500",
    })
    if err != nil {
        panic(err)
    }
    cs, err := consul.New(consulClient, consul.WithPath("app/cart/configs/"))
    //consul中需要标注文件后缀，kratos读取配置需要适配文件后缀
    //The file suffix needs to be marked, and kratos needs to adapt the file suffix to read the configuration.
    if err != nil {
        panic(err)
    }
    c := config.New(config.WithSource(cs))
}
```
