# lib/util/slices

[功能] 提供泛型切片筛选、转换、查询、删除和迭代。适用于共享业务集合算法。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/slices@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	slices "github.com/Iori372552686/GoOne/lib/util/slices"
)

func main() {
	values := []int{1, 2, 3, 4}
	fmt.Println(slices.FilterSlice(values, func(n int) bool { return n%2 == 0 }))
	fmt.Println(slices.Contains(values, 3))
}
```
