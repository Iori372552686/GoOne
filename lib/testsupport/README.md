# lib/testsupport

[功能] 为 GoOne 的中间件集成测试提供统一门控和 TCP 预检。未设置 `GOONE_INTEGRATION=1` 时会跳过，启用后地址不可达会使测试失败。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/testsupport@v0.1.0
```

## 示例

[运行条件] 保存为消费项目的 `example_test.go` 后执行 `go test`。设置 `GOONE_INTEGRATION=1` 时需要本地 Redis。

```go
package example

import (
	"github.com/Iori372552686/GoOne/lib/testsupport"
	"testing"
)

func TestRedisAvailability(t *testing.T) {
	testsupport.Require(t, "127.0.0.1:6379")
	// 在此使用已经通过预检的 Redis 服务。
}
```
