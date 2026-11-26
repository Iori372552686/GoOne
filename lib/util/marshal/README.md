# lib/util/marshal

[功能] 从文件加载 JSON、YAML 或按扩展名选择解析方式。使用 `LoadConfFile` 将配置读入调用方提供的结构体。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/marshal@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
	"log"
	"os"
)

func main() {
	f, err := os.CreateTemp("", "goone-example-*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("port: 8000\n"); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	var cfg struct{ Port int }
	if err := marshal.LoadConfFile(f.Name(), &cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Println(cfg.Port)
}
```
