# lib/db/ssdb

[功能] 为旧系统配置 SSDB 连接池。`NewSsdbEngine` 创建包装对象，`AddInstance` 返回可用的连接池。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/ssdb@v0.1.0
```

## 示例

[运行条件] 运行需要可达的 SSDB。使用结束后关闭 `AddInstance` 返回的池。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/ssdb"
	"log"
	"os"
)

func main() {
	pool, err := ssdb.NewSsdbEngine().AddInstance(ssdb.Config{
		IP: "127.0.0.1", Port: 8888, MaxPool: 10,
		Password: os.Getenv("SSDB_PASSWORD"),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	fmt.Printf("%T\n", pool)
}
```
