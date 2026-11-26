# lib/db/mysql

[功能] 以实例 ID 管理传统 `database/sql` MySQL 连接池，提供查询和执行入口。`AddInstance` 建立实例，`Destroy` 关闭连接。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/db/mysql@v0.1.0
```

## 示例

[运行条件] 运行需要本地 MySQL 和具有查询权限的 `demo` 账户。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/db/mysql"
	"log"
	"os"
)

func main() {
	manager := mysql.NewMysqlMgr()
	defer manager.Destroy()
	err := manager.AddInstance(1, "127.0.0.1", 3306, "demo", os.Getenv("MYSQL_PASSWORD"), "demo")
	if err != nil {
		log.Fatal(err)
	}
	rows, err := manager.Query(1, "SELECT 1")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			log.Fatal(err)
		}
		fmt.Println(value)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
```
