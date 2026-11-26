# lib/util/file

[功能] 提供目录判断和按后缀递归遍历文件。适用于加载资源或检查配置目录。入口包括 `IsDir`、`ListDir`、`ListAllFile`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/file@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/file"
	"log"
)

func main() {
	files, err := file.ListAllFile(".", ".go")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(files)
}
```
