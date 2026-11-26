# lib/db/dal

[功能] 用 `TieredStore` 编排 L2 读穿缓存、L3 持久化与回填。业务提供 `EntityCodec`、`L2Cache` 和 `L3Store` 的实现。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/dal@v0.1.0
```

## 示例

[运行条件] 示例用内存桩演示从 L3 回源。生产调用方负责连接池、缓存 TTL、持久化实现和退出清理。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/dal"
	"log"
)

type codec struct{}

func (codec) MarshalL3(value string) ([]byte, error)  { return []byte(value), nil }
func (codec) UnmarshalL3(data []byte) (string, error) { return string(data), nil }
func (codec) MarshalL2Fields(value string, _ []string) (map[string][]byte, error) {
	return map[string][]byte{"value": []byte(value)}, nil
}
func (codec) UnmarshalL2Fields(fields map[string][]byte) (string, error) {
	return string(fields["value"]), nil
}

type cache struct{}

func (cache) Load(context.Context, uint64) (map[string][]byte, bool, error) {
	return nil, false, nil
}
func (cache) Save(context.Context, uint64, map[string][]byte) error { return nil }
func (cache) Delete(context.Context, uint64) error                  { return nil }

type storage struct{}

func (storage) Load(context.Context, uint64) ([]byte, int64, error) {
	return []byte("sample"), 1, nil
}
func (storage) Save(context.Context, uint64, []byte, int64) error { return nil }
func (storage) Delete(context.Context, uint64) error              { return nil }

func main() {
	store := dal.NewTieredStore[string](codec{}, cache{}, storage{}, nil)
	value, found, err := store.Load(context.Background(), 17)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value, found)
}
```
