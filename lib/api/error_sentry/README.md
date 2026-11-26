# lib/api/error_sentry

[功能] 将应用错误上报到 Sentry 或 Bugsnag。Sentry 入口是 `InitSentry` 和 `CaptureErrorToSentry`，Bugsnag 入口是 `InitBugsnag` 和 `Error`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/api/error_sentry@v0.1.0
```

## 示例

[运行条件] 运行上报示例需要可用的 `SENTRY_DSN`。短进程退出前调用 `Flush`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"github.com/Iori372552686/GoOne/lib/api/error_sentry"
	"github.com/getsentry/sentry-go"
	"log"
	"os"
	"time"
)

func main() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Fatal("set SENTRY_DSN")
	}
	if err := error_sentry.InitSentry(dsn, "1.0.0", "demo", "demo@1.0.0", ""); err != nil {
		log.Fatal(err)
	}
	error_sentry.CaptureErrorToSentry(sentry.LevelError, "demo-user", "", "example error")
	sentry.Flush(2 * time.Second)
}
```
