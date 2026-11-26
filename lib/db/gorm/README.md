# lib/db/gorm

[功能] 管理命名的 GORM MySQL 实例、连接池、主从路由和事务。入口为 `NewManager`、`InitAndRun`、`GetDB`、`Transaction` 和 `Close`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/gorm@v0.1.0
```

## 示例

[运行条件] 运行需要本地 MySQL、`demo` 数据库和相应账户。通过 `MYSQL_PASSWORD` 提供密码。退出时关闭管理器。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/gorm"
	"log"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	manager := gormdb.NewManager()
	defer manager.Close()
	err := manager.InitAndRun(ctx, []gormdb.Config{{
		IndexName: "default",
		Master: &gormdb.DbInfo{
			IP: "127.0.0.1", Port: 3306, User: "demo",
			Password: os.Getenv("MYSQL_PASSWORD"), DBName: "demo",
		},
	}})
	if err != nil {
		log.Fatal(err)
	}
	db, err := manager.GetDB("default")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(db.Dialector.Name())
}
```
