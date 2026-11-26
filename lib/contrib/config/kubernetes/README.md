# lib/contrib/config/kubernetes

[功能] 把 Kubernetes ConfigMap 用作配置源。用 `NewSource` 指定命名空间与选择器，再用 `Load` 或 `Watch`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/contrib/config/kubernetes@v0.1.0
```

## 示例

[运行条件] 运行前在集群中配置对应标签的 ConfigMap，并提供 list/watch 权限。在集群外填写 `KUBECONFIG`。订阅结束时停止 watcher。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/contrib/config/kubernetes"
	"log"
	"os"
)

func main() {
	source := kubernetes.NewSource(
		kubernetes.KubeConfig(os.Getenv("KUBECONFIG")),
		kubernetes.Namespace("default"),
		kubernetes.LabelSelector("app=demo"),
	)
	items, err := source.Load()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(items))
}
```

---

# Kubernetes Config

### Usage in the Kubernates Cluster
It is required to 
> serviceaccount should be set to the actual account of your environment, the default account will be `namespace::default` if the `spec.serviceAccount` is unset. 
execute this command:
```
kubectl create clusterrolebinding go-kratos:kube --clusterrole=view --serviceaccount=mesh:default
```
or use `kubect apply -f bind-role.yaml`
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: go-kratos:kube
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
- kind: ServiceAccount
  name: default
  namespace: mesh
```

### Usage outside the Kubernates Cluster
Set the path `~/.kube/config` to KubeConfig
```go
    config.NewSource(SourceOption{
		Namespace:     "mesh",
		LabelSelector: "",
		KubeConfig:    filepath.Join(homedir.HomeDir(), ".kube", "config"),
	})
```
