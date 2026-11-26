# lib/web/http_client

[功能] 保留旧 Web HTTP 调用路径。`GetRequest`、`HttpRequestByHeader` 等入口委托给独立的 `lib/api/http_client` 模块。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/web/http_client@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/web/http_client"
	"log"
	"net/http"
	"net/http/httptest"
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	body, err := http_client.GetRequest(server.URL)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}
```
