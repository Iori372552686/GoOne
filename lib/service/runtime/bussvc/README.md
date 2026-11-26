# lib/service/runtime/bussvc

[功能] 装配标准 GoOne 服务组件，包括配置加载、时间刷新、日志、就绪检查、追踪和可选消息路由。使用 `Assembly` 注入应用的配置读取和路由规则，再按依赖顺序注册业务组件。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/runtime/bussvc@v0.1.0
```

## 示例

[运行条件] 示例只装配。给 `Assembly` 配置 `LoadServiceConfig` 和 `CommonForService` 后运行应用。需要消息路由时显式注册 `NewRouterComponent`，传入事务管理器、选定的 MQ 驱动注册表及配置中的注册中心地址。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
)

func main() {
	assembly := bussvc.Assembly{
		CommonForService: func(string) bussvc.Common {
			return bussvc.Common{LogLevel: "info"}
		},
	}
	app := assembly.MustNew("example", nil, assembly.WithConfLoader())
	fmt.Println(app.Name())
}
```
