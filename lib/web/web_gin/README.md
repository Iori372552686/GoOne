# lib/web/web_gin

[功能] 装配 Gin 路由、CORS、请求限制与 HTTP 服务。使用 `NewServer` 构造，`StartGin` 启动监听并返回运行期错误通道。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/web/web_gin@v0.1.0
```

## 示例

[运行条件] 示例只构造服务。需要监听时用 `StartGin`，退出时对返回的 HTTP server 执行 `Shutdown`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

func main() {
	server, err := web_gin.NewServer(web_gin.Config{
		IP: "127.0.0.1", Port: 8080, Mode: gin.ReleaseMode,
		ReadHeaderTimeout: 2 * time.Second, MaxBodyBytes: 1 << 20,
	}, func(router *gin.Engine) {
		router.GET("/health", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "ok")
		})
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(server.Addr)
}
```
