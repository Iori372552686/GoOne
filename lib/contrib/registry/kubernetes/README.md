# lib/contrib/registry/kubernetes

[功能] 通过 Kubernetes Pod 信息注册和发现服务。入口包括 `NewRegistry`、`Start`、`Service`、`Watch` 和 `Close`。

## 安装

[版本] 需要 Go 1.26.0 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/registry/kubernetes@v0.1.0
```

## 示例

[运行条件] 运行需要有效集群配置和 Pod 的 list/watch 权限。注册时还需修改目标 Pod 元数据的权限。请为示例指定 `KUBECONFIG`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	kubernetes "github.com/Iori372552686/GoOne/lib/contrib/registry/kubernetes"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
)

func main() {
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Fatal(err)
	}
	client, err := kubeclient.NewForConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	r := kubernetes.NewRegistry(client)
	r.Start()
	defer r.Close()
	instances, err := r.Service("inventory")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(instances))
}
```
