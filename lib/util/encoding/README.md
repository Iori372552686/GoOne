# lib/util/encoding

[功能] 按媒体子类型注册和取得编解码器。用 `GetCodec` 取实例，通过导入所需子包注册 JSON、YAML、TOML、XML、MessagePack、Protobuf。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/encoding@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/encoding"
	_ "github.com/Iori372552686/GoOne/lib/util/encoding/json"
	"log"
)

func main() {
	codec := encoding.GetCodec("json")
	raw, err := codec.Marshal(map[string]string{"status": "ok"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(raw))
}
```
