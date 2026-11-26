# lib/web/rest_api

[功能] 管理具名 REST 后端、请求超时和可选签名。`NewRestApi` 创建实例，`Get`、`Post`、`SignedGet` 和 `SignedPost` 发起请求。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/web/rest_api@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"log"
	"net/http"
	"net/http/httptest"
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()
	api := rest_api.NewRestApi(rest_api.Config{
		ServiceName: "demo", Urls: []string{server.URL + "?"},
	}, nil)
	body, err := api.Get(context.Background(), 17, map[string]string{"kind": "test"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(body))
}
```
