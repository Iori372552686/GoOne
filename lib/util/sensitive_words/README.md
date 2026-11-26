# lib/util/sensitive_words

[功能] 从文件加载词表并执行匹配和替换。先调用 `Init`，再用 `ChangeSensitiveWords` 检查输入文本。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/sensitive_words@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"log"
	"os"
)

func main() {
	f, err := os.CreateTemp("", "goone-words-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("sample\n"); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	sensitive_words.Init(f.Name())
	found, output := sensitive_words.ChangeSensitiveWords("a sample word")
	fmt.Println(found, output)
}
```
