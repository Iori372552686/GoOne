# lib/util/crypto

[功能] 提供 Base64、MD5 和既有 AES、XXTEA 格式的兼容工具。`aes`、`xxtea` 随本模块发布。MD5 用途是兼容现有摘要或校验值。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/crypto@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/crypto"
	"log"
)

func main() {
	encoded := crypto.Base64EncodeStr([]byte("sample"))
	decoded, err := crypto.Base64DecodeStr(encoded)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(encoded, string(decoded))
}
```
