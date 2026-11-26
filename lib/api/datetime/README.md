# lib/api/datetime

[功能] 提供缓存时钟、Unix 时间戳、时区换算和日历判断。服务可用 `DefaultDateTimeTick` 注册周期刷新任务，独立调用可显式执行 `Tick`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/datetime@v0.1.0
```

## 示例

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
)

func main() {
	datetime.Tick()
	fmt.Println(datetime.NowInt64(), datetime.NowMs())
}
```
