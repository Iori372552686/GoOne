package bus

import (
	"fmt"
	"testing"
	"time"
)

func onRecvMsg(srcBusID uint32, data []byte) {
	fmt.Printf("srcBusID:%v, data:%v", srcBusID, data)
}

func TestBus(t *testing.T) {
	impl := CreateBus("rabbitmq", 1, onRecvMsg, "amqp://guest:guest@localhost:5672/")
	if impl == nil {
		return
	}

	impl.Send(impl.SelfBusId(), []byte("abc"), nil)

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
	}

}
