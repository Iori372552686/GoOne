// Package rabbitmq provides the RabbitMQ bus driver.
// Import it (usually via driver/all) to enable "amqp://" / "rabbitmq://" addresses.
package rabbitmq

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/internal/wire"
	amqp "github.com/rabbitmq/amqp091-go"
)

type BusImplRabbitMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan wire.OutMsg
	onRecv    bus.MsgHandler
	stopCh    chan struct{}
	closed    atomic.Bool
	connected atomic.Bool
	closeOnce sync.Once
}

func NewBusImplRabbitMQ(selfBusId uint32, onRecvMsg bus.MsgHandler, addr string) *BusImplRabbitMQ {
	impl := new(BusImplRabbitMQ)
	impl.selfBusId = selfBusId
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan wire.OutMsg, 10000)
	impl.onRecv = onRecvMsg
	impl.stopCh = make(chan struct{})
	go impl.run(addr)
	return impl
}

func (b *BusImplRabbitMQ) SelfBusId() uint32 {
	return b.selfBusId
}

func (b *BusImplRabbitMQ) SetReceiver(onRecvMsg bus.MsgHandler) {
	b.onRecv = onRecvMsg
}

func (b *BusImplRabbitMQ) Healthy() bool {
	return b.connected.Load() && !b.closed.Load()
}

func (b *BusImplRabbitMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	msg := wire.OutMsg{
		BusID: dstBusId,
		Data:  wire.BuildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !wire.SendToMsgChan(b.chanOut, msg, b.timeout) {
		wire.PutFrameBuf(msg.Data)
		return fmt.Errorf("bus.chanOut<-msg time out")
	} // msg所有权已转移，后面不能再使用msg

	return nil
}

func (b *BusImplRabbitMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)
	})
	return nil
}

func (b *BusImplRabbitMQ) process(rabbitmqAddr string, myQueueName string) error {
	conn, err := amqp.Dial(rabbitmqAddr)
	if err != nil {
		return fmt.Errorf("failed to connect MQ {addr:%v} | %v", rabbitmqAddr, err)
	}
	defer conn.Close()
	logger.Infof("connected to %v", rabbitmqAddr)

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel | %v", err)
	}
	defer ch.Close()

	queueArguments := amqp.Table{ // arguments
		"x-message-ttl":      int32(30 * 60 * 1000),
		"x-max-length-bytes": int32(10 * 1024 * 1024),
		"x-overflow":         "reject-publish",
	}
	q, err := ch.QueueDeclare(myQueueName, false, false, false, false, queueArguments)
	if err != nil {
		return fmt.Errorf("failed to declare a queue | %v", err)
	}

	chanRecv, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to register a consumer | %v", err)
	}

	b.connected.Store(true)
	defer b.connected.Store(false)

	for {
		select {
		case <-b.stopCh:
			return nil
		case msgOut, ok := <-b.chanOut:
			if !ok {
				return fmt.Errorf("chanOut of bus is closed")
			}

			// send by routering
			err = ch.Publish(
				"",                               // exchange
				wire.CalcQueueName(msgOut.BusID), // routing key
				false,                            // mandatory
				false,                            // immediate
				amqp.Publishing{
					// ContentType: "text/plain",
					Body: msgOut.Data,
				})
			if err != nil {
				logger.Errorf("Failed to publish a message {busId:%v, dataLen:%v}| %v", msgOut.BusID, len(msgOut.Data), err)
				// todo: is it necessary to return the err?
			}
			wire.PutFrameBuf(msgOut.Data) // Publish 已同步序列化到连接缓冲，可安全回收
		case delivery, ok := <-chanRecv:
			if !ok {
				return fmt.Errorf("chanRecv of bus is closed")
			}
			if len(delivery.Body) < wire.HeaderLen() {
				logger.Warningf("Received a too short rabbitmq bus message {len:%v, expect:%v}",
					len(delivery.Body), wire.HeaderLen())
				continue
			}

			header := wire.Header{}
			header.From(delivery.Body)
			//logger.Debugf("Received message from MQ: %+v", header)
			if header.PassCode != wire.PassCode {
				logger.Warningf("Received a bus message with wrong pass code: %#v", header)
				continue
			}

			if b.onRecv != nil {
				// streadway/amqp 的 delivery.Body 由 channel 在 recvContent 中以
				// append(ch.body, frame.Body...) 累积，下一帧到来时 ch.body 被
				// make([]byte,0) 新分配重置（非复用旧底层数组），因此 delivery.Body
				// 在 dispatch 后可被安全持有。这里直接切片共享，无需防御性 copy。
				// 上游 router.onRecvBusMsg 对 body 仅做切片引用。
				b.onRecv(header.SrcBusID, delivery.Body[wire.HeaderLen():])
			}
		}
	}
}

func (b *BusImplRabbitMQ) run(rabbitmqAddr string) {
	myQueueName := wire.CalcQueueName(b.selfBusId)
	logger.Errorf("Start bus service {myQueueName:%s}", myQueueName)

	retryCount := 0
	for {
		if b.closed.Load() {
			return
		}
		processStartTime := time.Now()

		err := b.process(rabbitmqAddr, myQueueName)
		if b.closed.Load() {
			return
		}

		if time.Now().Sub(processStartTime) > time.Minute {
			retryCount = 0 // 正常运行1分钟以上，则重置retryCount
		}
		retryCount++
		retryAfterSeconds := (retryCount - 1) * 2
		if retryAfterSeconds > 30 {
			retryAfterSeconds = 30
		}
		logger.Errorf("Error occur in processing bus. Retry later {retryTimes: %v, afterSeconds:%v} | %v", retryCount, retryAfterSeconds, err)
		if !wire.SleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

// DriverName 是本 driver 注册时使用的 implType 字符串。
const DriverName = "rabbitmq"

// newBus 是具体 ctor，同时被遗留的 func init 注册与显式 Driver() 描述符共用，使两
// 条路径不会分叉。
func newBus(selfBusId uint32, onRecvMsg bus.MsgHandler, conf any) (bus.IBus, error) {
	switch v := conf.(type) {
	case string:
		if v == "" {
			return nil, fmt.Errorf("rabbitmq addr is empty")
		}
		return NewBusImplRabbitMQ(selfBusId, onRecvMsg, v), nil
	case bus.RabbitMQConfig:
		if v.Addr == "" {
			return nil, fmt.Errorf("rabbitmq addr is empty")
		}
		return NewBusImplRabbitMQ(selfBusId, onRecvMsg, v.Addr), nil
	default:
		return nil, fmt.Errorf("rabbitmq unsupported config type %T", conf)
	}
}

// Driver 返回用于装配期注册的显式 driver 描述符。只想链接本 driver 的应用，应通过
// bus.DriverRegistry.MustRegister(rabbitmq.Driver()) 注册，而非 blank-import
// driver/all。
func Driver() bus.Driver {
	return bus.Driver{Name: DriverName, Ctor: newBus}
}
