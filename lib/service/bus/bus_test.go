package bus

import (
	"log"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/bus"
)

func onRecvMsg(srcBusID uint32, data []byte) error {
	log.Printf("srcBusID:%v, data:%v", srcBusID, data)

	return nil
}

func TestBus(t *testing.T) {
	impl := CreateBus("rabbitmq", bus.IpStringToInt("1.1.2.2"), onRecvMsg, "amqp://guest:guest@192.168.50.11:5672/")
	if impl == nil {
		return
	}

	impl.Send(impl.SelfBusId(), []byte("abc"), nil)

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
	}

}

func TestNsqBus(t *testing.T) {
	conf := Config{
		[]string{"db-cfg-center.miniworldplus.com:4161", "db-cfg-center.miniworldplus.com:4161"},
		"db-cfg-center.miniworldplus.com",
		4150,
		"test",
		"ch",
		3,
	}

	impl := NewBusImplNsqMQ(1, onRecvMsg, conf)
	if impl == nil {
		return
	}

	for i := 0; i < 10; i++ {
		impl.SendTo("test", []byte("abc"), []byte("123"))
		time.Sleep(1 * time.Second)
	}
}
