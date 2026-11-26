# lib/contrib/config/apollo

[功能] 读取和监听 Apollo 应用配置。用 `NewSourceE` 创建配置源并检查启动错误，可指定应用、集群和命名空间。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/apollo@v0.1.0
```

## 示例

[运行条件] 运行需要可用的 Apollo 配置服务和已发布命名空间。调用 `Watch` 后在退出时用 `Stop` 停止监听。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/apollo"
	"log"
)

func main() {
	source, err := apollo.NewSourceE(
		apollo.WithEndpoint("http://127.0.0.1:8080"),
		apollo.WithAppID("demo"),
		apollo.WithNamespace("application"),
		apollo.WithDisableBackup(),
	)
	if err != nil {
		log.Fatal(err)
	}
	items, err := source.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(items))
}
```

---

## Apollo config centry

This module implements the local `config.Source` interface based on apollo config management center.

### Quick start

```go
import (
	"fmt"
	"zap"

	"github.com/Iori372552686/GoOne/lib/contrib/config/apollo"
)

func main() {
	src := apollo.NewSource(
		apollo.WithAppID("kratos"),
		apollo.WithCluster("dev"),
		apollo.WithEndpoint("http://localhost:8080"),
		apollo.WithNamespace("application,event.yaml,demo.json"),
		apollo.WithEnableBackup(),
		apollo.WithSecret("ad75b33c77ae4b9c9626d969c44f41ee"),
	)
	kvs, err := src.Load()
	if err != nil {
		panic(err)
	}
	_ = kvs
	// use watch operations，help yourself.
}
```

### Options list

> You get what you see.

```go
// specify the app id
func WithAppID(appID string) Option
// specify the cluster of application
func WithCluster(cluster string) Option

// enable backup or not, and where to back up them.
func WithBackupPath(backupPath string) Option
func WithDisableBackup() Option
func WithEnableBackup() Option

// specify apollo endpoint, such as http://localhost:8080
func WithEndpoint(endpoint string) Option

// inject a logger to debug
func WithLogger(logger Logger) Option

// namespaces to load, comma to separate. 
func WithNamespace(name string) Option

// secret is the apollo secret key to access application config.
func WithSecret(secret string) Option
```

### Notice

apollo config center use `Namespace` to be part of the key. For example:

***application.json***

```json
{
  "http": {
    "address": ":8080",
    "tls": {
      "enable": false,
      "cert_file": "",
      "key_file": ""
    }
  }
}
```

you got them in local config kv maybe look like:

```go
config := map[string]interface{}{
	// application be part of the key path.
	"application": map[string]interface{}{
        "http": map[string]interface{}{
            "address": ":8080",
            "tls": map[string]interface{}{
                "enable": false,
                "cert_file": "",
                "key_file": ""
            }
        }
    }
}
_ = config
```
