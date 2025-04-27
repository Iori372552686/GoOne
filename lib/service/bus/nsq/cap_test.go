package nsq

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"log"
	"math/rand"
	"testing"
	"time"
)

func TestConsumer(t *testing.T) {
	_, err := NewConsumer("test", "ch1", "nacos.miniworldplus.com:4161", []string{}, 3, nil)
	if err != nil {
		logger.Errorf("init Consumer error")
	}
	_, err = NewConsumer("test", "ch1", "nacos.miniworldplus.com:4161", []string{}, 3, nil)
	if err != nil {
		logger.Errorf("init Consumer error")
	}
	select {}
}

func TestProducer(t *testing.T) {
	producer, err := NewProducer("nacos.miniworldplus.com:4150")
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
		err = producer.Publish("test", buf)
		if err != nil {
			log.Panic(err)
		}
		time.Sleep(time.Second * 1)
	}

	producer.Stop()
}
