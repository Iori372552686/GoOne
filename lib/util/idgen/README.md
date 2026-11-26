# lib/util/idgen

[功能] 用 Sonyflake 生成 64 位 ID。使用 `NewIDGen` 初始化，再用 `GenID` 获取 ID，并处理节点初始化和生成错误。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/idgen@v0.1.0
```

## 示例

[运行条件] 机器标识来自本机网络地址相关信息。跨实例部署时需核对机器标识是否会冲突。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"log"
)

func main() {
	generator, err := idgen.NewIDGen()
	if err != nil {
		log.Fatal(err)
	}
	id, err := generator.GenID()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}
```
