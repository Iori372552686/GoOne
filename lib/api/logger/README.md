# lib/api/logger

[功能] 基于 Zap 提供日志初始化、分级输出和命令字日志过滤。`plug` 与 `zap` 子包随本模块发布。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/logger@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"log"
	"os"
)

func main() {
	dir, err := os.MkdirTemp("", "goone-log-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if _, err := logger.InitLogger(dir, "info", "demo"); err != nil {
		log.Fatal(err)
	}
	defer logger.Flush()
	logger.Infof("service %s started", "demo")
}
```
