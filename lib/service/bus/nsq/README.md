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