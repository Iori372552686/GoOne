package nsq

import (
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// TestConsumer is a manual integration test: it requires an external nsq
// lookupd and never terminates by itself, so it is skipped by default.
func TestConsumer(t *testing.T) {
	t.Skip("manual integration test; requires external nsqlookupd")

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

// TestProducer is a manual integration test: it requires an external nsqd,
// so it is skipped by default.
func TestProducer(t *testing.T) {
	t.Skip("manual integration test; requires external nsqd")

	producer, err := NewProducer("nacos.miniworldplus.com:4150")
	if err != nil {
		log.Panic(err)
	}
	defer producer.Stop()

	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	for n := 0; n < 10; n++ {
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
}
