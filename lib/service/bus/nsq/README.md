# lib/service/bus/nsq

[功能] 为直接使用 NSQ 的项目提供生产者和消费者包装。生产者用 `NewProducer`，消费者用 `NewConsumer` 或 `NewConsumerWithOpts`。

## 安装

[版本] 需要 Go 1.25.4 或更新版本。以下命令在模块的首发标签发布后可执行。

```sh
go get github.com/Iori372552686/GoOne/lib/service/bus/nsq@v0.1.0
```

## 示例

[运行条件] 运行需要 nsqd。消费者结束时也应调用其 `Stop`。GoOne 总线驱动位于 `lib/service/bus/driver/nsq`。

[用法] 将示例保存为消费者模块中的 `main.go`，执行 `go run .`。

```go
package main

import (
	"github.com/Iori372552686/GoOne/lib/service/bus/nsq"
	"log"
)

func main() {
	producer, err := nsq.NewProducer("127.0.0.1:4150")
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Stop()
	if err := producer.Public("demo", "hello"); err != nil {
		log.Fatal(err)
	}
}
```

---

##### make by Iori

#nsq git地址： 
```
github.com/nsqio/go-nsq
```

## 生产者示例
```
package main

import (
"github.com/nsqio/go-nsq"
"log"
"math/rand"
"time"
)

func main() {
config := nsq.NewConfig()
w, err := nsq.NewProducer("127.0.0.1:4150", config)

	if err != nil {
		log.Panic(err)
	}

	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	for {
		buf := make([]byte, 4)
		for i := 0; i < 4; i++ {
			buf[i] = chars[rand.Intn(len(chars))]
		}
		log.Printf("Pub: %s", buf)
		err = w.Publish("test", buf)
		if err != nil {
			log.Panic(err)
		}
		time.Sleep(time.Second * 1)
	}

	w.Stop()
}
```



##消费者示例
```
package main

import (
"log"
"sync"

	"github.com/nsqio/go-nsq"
)

func main() {

	wg := &sync.WaitGroup{}
	wg.Add(1000)

	config := nsq.NewConfig()
	q, _ := nsq.NewConsumer("test", "ch", config)
	q.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		log.Printf("Got a message: %s", message.Body)
		wg.Done()
		return nil
	}))
	err := q.ConnectToNSQD("127.0.0.1:4150")
	if err != nil {
		log.Panic(err)
	}
	wg.Wait()

}
```
