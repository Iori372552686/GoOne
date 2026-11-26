# lib/util/zip

[功能] 提供项目使用的 zlib 和 gzip 字节压缩、解压工具。常用入口有 `ZipEncode`、`ZipDecode`、`GzipEncode` 和 `GzipDecode`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/zip@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/zip"
	"log"
)

func main() {
	compressed := zip.ZipEncode([]byte("payload"))
	decoded, err := zip.ZipDecode(compressed)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(decoded))
}
```
