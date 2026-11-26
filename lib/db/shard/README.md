# lib/db/shard

[功能] 用规则对象把业务主键映射到 ORM 实例和物理表，并提供 GORM 建表、迁移辅助入口 `EnsureTables` 和 `MigrateUnsharded`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/shard@v0.1.0
```

## 示例

[运行条件] 路由示例无需数据库。建表和迁移入口需要由调用方注入 GORM 连接，变更既有分表规则前要安排数据迁移。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/shard"
	"log"
)

func main() {
	rule := shard.Rule{Name: "roles", Instance: "default", TableBase: "roles", TableShards: 16}
	target, err := rule.Resolve(shard.RouteParams{Uid: 1234})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(target.Instance, target.Table)
}
```
