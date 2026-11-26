# lib/service/transaction

[功能] 按 Router ID 或 UID 串行处理同键请求，并用分片队列和在途限制提供并发和背压。已生成的 SSRPC 绑定可用于注册事务处理器。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/transaction@v0.1.0
```

## 示例

[运行条件] 启动前完成处理器注册。退出前调用 `Close`，服务可传入带截止时间的 context。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"context"
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"log"
)

func main() {
	mgr := transaction.NewTransactionMgr()
	mgr.InitAndRunWithConfig(transaction.TransactionMgrConfig{MaxTrans: 128})
	fmt.Println(mgr.StatsSnapshot().ShardCount)
	if err := mgr.Close(context.Background()); err != nil {
		log.Fatal(err)
	}
}
```
