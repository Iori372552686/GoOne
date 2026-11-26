# lib/service/ssrpc

[功能] 注册并派发 GoOne CMD、WebSocket、HTTP 与 gRPC 处理器，提供中间件、错误映射及追踪。IDL 调用方通常用生成的注册函数和 `RegistryComponent`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/ssrpc@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"net/http/httptest"
)

func main() {
	registry := ssrpc.NewRegistry()
	err := registry.Register("health", ssrpc.Binding{
		Kind: ssrpc.BindingHTTP, HTTPMethod: http.MethodGet, HTTPPath: "/health",
		HTTPHandler: func(ctx *gin.Context) { ctx.String(http.StatusOK, "ok") },
	})
	if err != nil {
		log.Fatal(err)
	}
	dispatcher, err := registry.Seal()
	if err != nil {
		log.Fatal(err)
	}
	engine := gin.New()
	dispatcher.MountGin(engine)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	fmt.Println(response.Code, response.Body.String())
}
```
