// Package rocketmq provides the RocketMQ bus driver.
// Import it (usually via driver/all) to enable "rocketmq://" addresses.
package rocketmq

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/internal/wire"

	rmq "github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

type BusImplRocketMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan wire.OutMsg
	chanIn    chan []byte
	onRecv    bus.MsgHandler

	nameServers   []string
	topic         string
	consumerGroup string
	stopCh        chan struct{}
	closed        atomic.Bool
	connected     atomic.Bool
	closeOnce     sync.Once
}

func NewBusImplRocketMQ(selfBusId uint32, onRecvMsg bus.MsgHandler, conf bus.RocketMQConfig) bus.IBus {
	topic := strings.TrimSpace(conf.Topic)
	if topic == "" {
		topic = "goone_bus"
	}
	group := strings.TrimSpace(conf.ConsumerGroup)
	if group == "" {
		group = "goone_bus"
	}
	impl := &BusImplRocketMQ{
		selfBusId:     selfBusId,
		timeout:       3 * time.Second,
		chanOut:       make(chan wire.OutMsg, 10000),
		chanIn:        make(chan []byte, 10000),
		onRecv:        onRecvMsg,
		nameServers:   conf.NameServers,
		topic:         topic,
		consumerGroup: group,
		stopCh:        make(chan struct{}),
	}
	go impl.run()
	return impl
}

func (b *BusImplRocketMQ) SelfBusId() uint32                    { return b.selfBusId }
func (b *BusImplRocketMQ) SetReceiver(onRecvMsg bus.MsgHandler) { b.onRecv = onRecvMsg }
func (b *BusImplRocketMQ) Healthy() bool                        { return b.connected.Load() && !b.closed.Load() }

func (b *BusImplRocketMQ) tagFor(busId uint32) string {
	return wire.CalcQueueName(busId)
}

func (b *BusImplRocketMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	msg := wire.OutMsg{
		BusID:  dstBusId,
		Topics: b.tagFor(dstBusId), // tag
		Data:   wire.BuildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !wire.SendToMsgChan(b.chanOut, msg, b.timeout) {
		wire.PutFrameBuf(msg.Data)
		return fmt.Errorf("rocketmq bus.chanOut<-msg time out")
	}
	return nil
}

func (b *BusImplRocketMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)
	})
	return nil
}

func (b *BusImplRocketMQ) process() error {
	if len(b.nameServers) == 0 {
		return fmt.Errorf("rocketmq nameservers is empty")
	}
	ctx := context.Background()

	p, err := rmq.NewProducer(producer.WithNameServer(b.nameServers))
	if err != nil {
		return err
	}
	if err := p.Start(); err != nil {
		return err
	}
	defer func() { _ = p.Shutdown() }()

	c, err := rmq.NewPushConsumer(
		consumer.WithNameServer(b.nameServers),
		consumer.WithGroupName(b.consumerGroup+"."+wire.CalcQueueName(b.selfBusId)),
	)
	if err != nil {
		return err
	}

	tagExpr := b.tagFor(b.selfBusId)
	if err := c.Subscribe(b.topic, consumer.MessageSelector{Type: consumer.TAG, Expression: tagExpr},
		func(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
			for _, m := range msgs {
				if m == nil {
					continue
				}
				buf := make([]byte, len(m.Body))
				copy(buf, m.Body)
				select {
				case b.chanIn <- buf:
				case <-b.stopCh:
					return consumer.ConsumeSuccess, nil
				}
			}
			return consumer.ConsumeSuccess, nil
		}); err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	defer func() { _ = c.Shutdown() }()

	logger.Infof("RocketMQ bus started {topic:%s, tag:%s}", b.topic, tagExpr)

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
			_, err := p.SendSync(ctx, primitive.NewMessage(b.topic, msgOut.Data).WithTag(msgOut.Topics))
			if err != nil {
				logger.Errorf("Failed to publish rocketmq message {topic:%v, tag:%v, dataLen:%v}| %v",
					b.topic, msgOut.Topics, len(msgOut.Data), err)
			}
			wire.PutFrameBuf(msgOut.Data) // SendSync 已同步完成，可安全回收
		case data, ok := <-b.chanIn:
			if !ok {
				return fmt.Errorf("chanIn of bus is closed")
			}
			if len(data) < wire.HeaderLen() {
				continue
			}
			header := wire.Header{}
			header.From(data)
			if header.PassCode != wire.PassCode {
				logger.Warningf("Received a bus message with wrong pass code: %#v", header)
				continue
			}
			if b.onRecv != nil {
				// data 来自 chanIn，入队时已独立 make+copy（SDK 回调的 m.Body 生命周期
				// 不确定），取出后仅当前 goroutine 持有，可直接切片共享，无需再 copy。
				b.onRecv(header.SrcBusID, data[wire.HeaderLen():])
			}
		}
	}
}

func (b *BusImplRocketMQ) run() {
	retryCount := 0
	for {
		if b.closed.Load() {
			return
		}
		processStartTime := time.Now()
		err := b.process()
		if b.closed.Load() {
			return
		}
		if time.Since(processStartTime) > time.Minute {
			retryCount = 0
		}
		retryCount++
		retryAfterSeconds := (retryCount - 1) * 2
		if retryAfterSeconds > 30 {
			retryAfterSeconds = 30
		}
		logger.Errorf("Error occur in processing bus(rocketmq). Retry later {retryTimes: %v, afterSeconds:%v} | %v",
			retryCount, retryAfterSeconds, err)
		if !wire.SleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

// DriverName 是本 driver 注册时使用的 implType 字符串。
const DriverName = "rocketmq"

// newBus 是具体 ctor，同时被遗留的 func init 注册与显式 Driver() 描述符共用。
func newBus(selfBusId uint32, onRecvMsg bus.MsgHandler, conf any) (bus.IBus, error) {
	cfg, ok := conf.(bus.RocketMQConfig)
	if !ok {
		return nil, fmt.Errorf("rocketmq arg must be RocketMQConfig")
	}
	return NewBusImplRocketMQ(selfBusId, onRecvMsg, cfg), nil
}

// Driver 返回用于装配期注册的显式 driver 描述符。
func Driver() bus.Driver {
	return bus.Driver{Name: DriverName, Ctor: newBus}
}

func init() {
	bus.RegisterBus(DriverName, newBus)
}
