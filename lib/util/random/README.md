# lib/util/random

[功能] 生成伪随机整数、排列、字符串和字节，适合模拟和非安全业务采样。需要密码学随机值时使用标准库 `crypto/rand`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/random@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/random"
)

func main() {
	fmt.Println(random.Intn(6)+1, random.GetString(8))
}
```
