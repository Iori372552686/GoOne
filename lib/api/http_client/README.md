# lib/api/http_client

[功能] 提供支持 context、超时、响应体上限和自定义请求头的 HTTP 客户端。入口是 `NewClient` 和 `DoRequest`，旧调用可继续用 `GetRequest`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/http_client@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/http_client"
	"log"
	"net/http"
	"net/http/httptest"
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	client := http_client.NewClient(server.Client(), http_client.WithMaxResponseBytes(4096))
	body, err := client.DoRequest(context.Background(), http.MethodGet, server.URL, "", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}
```
