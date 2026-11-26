# lib/util/convert

[功能] 提供 GoOne 既有的整数、字节和 JSON 转换。需要 JSON 数字保持精度时使用 `JsonToMapUseNumber` 或按项目语义使用 `JsonToMapUseInt64`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/convert@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"log"
)

func main() {
	var record map[string]any
	err := convert.JsonToMapUseNumber([]byte("{\"uid\":9007199254740993}"), &record)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(record["uid"])
}
```
