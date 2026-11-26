# lib/util/generic

[功能] 定义整数类型约束 `Int` 和泛型迭代接口 `Iter[T]`，供泛型算法、容器和切片适配器复用。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/generic@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/generic"
)

func sum[T generic.Int](values []T) T {
	var total T
	for _, value := range values {
		total += value
	}
	return total
}

func main() {
	fmt.Println(sum([]int64{1, 2, 3}))
}
```
