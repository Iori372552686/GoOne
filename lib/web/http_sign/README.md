# lib/web/http_sign

[功能] 生成和校验包含参数、请求体和有效期的 HTTP 签名。`BuildHttpSign` 创建签名器，`PushSign` 生成签名，`CheckSign` 校验。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/web/http_sign@v0.1.0
```

## 示例

[运行条件] 示例密钥仅用于演示。实际服务从部署环境读取共享密钥。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"log"
)

func main() {
	const secret = "example-key"
	signer := http_sign.BuildHttpSign("", secret, 60, "", "", "2").
		WithSignType(http_sign.Sign_HMacSha256)
	body := []byte("payload")
	params := signer.PushSign(map[string]string{"player": "17"}, body)
	code, err := signer.CheckSign(params, body, params[http_sign.Const_SignName])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(code == http_sign.SignOK)
}
```
