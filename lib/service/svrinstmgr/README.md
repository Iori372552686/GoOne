# lib/service/svrinstmgr

[功能] 注册当前服务、订阅集群实例，并按随机、取模、一致性哈希或主节点规则选择实例。使用 `ServerInstanceMgr.InitAndRun` 和 `Close`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/svrinstmgr@v0.1.0
```

## 示例

[运行条件] 运行需要可访问的注册中心。实例列表由 watcher 异步更新。退出时关闭管理器。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/service/svrinstmgr"
	"log"
)

func main() {
	var mgr svrinstmgr.ServerInstanceMgr
	defer mgr.Close()
	rules := map[uint32]uint32{2: svrinstmgr.SvrRouterRule_Hash_UID}
	if err := mgr.InitAndRun("1.2.1.1", rules, "zk://127.0.0.1:2181"); err != nil {
		log.Fatal(err)
	}
	fmt.Println(mgr.GetAllSvrInsBySvrType(2))
}
```
