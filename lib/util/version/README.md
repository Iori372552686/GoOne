# lib/util/version

[功能] 处理项目既有的数字版本编码。`VersionToNum` 将版本字符串转换为数字，`NumToVersion` 输出版本字符串。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/version@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	version "github.com/Iori372552686/GoOne/lib/util/version"
)

func main() {
	number := version.VersionToNum("1.2.3")
	fmt.Println(number, version.NumToVersion(int(number)))
}
```
