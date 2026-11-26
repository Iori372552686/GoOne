# lib/util/lru

[功能] 维护固定容量的最近最少使用缓存。`Set` 写入，`Get` 查找并刷新访问顺序，`Peek` 查询但保留原顺序。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/util/lru@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/util/lru"
	"log"
)

func main() {
	cache := lru.NewLRUCache(128)
	if err := cache.Set("role:17", "online"); err != nil {
		log.Fatal(err)
	}
	value, found, err := cache.Get("role:17")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value, found)
}
```
