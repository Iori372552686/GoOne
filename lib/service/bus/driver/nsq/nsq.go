// Package nsq provides the NSQ bus driver.
// Import it (usually via driver/all) to enable "nsq://" addresses.
package nsq

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/internal/wire"
	nsqhelper "github.com/Iori372552686/GoOne/lib/service/bus/nsq"
)

/*
*  BusImplNsqMQ
*  @Description:
 */
type BusImplNsqMQ struct {
	selfBusId   uint32
	lookupAddr  []string
	NsqdAddr    string
	topicPrefix string
	chanName    string
	concurrency int
	maxInFlight          int
	lookupdPollInterval  time.Duration

	timeout   time.Duration
	chanOut   chan wire.OutMsg
	chanIn    chan []byte
	onRecv    bus.MsgHandler
	stopCh    chan struct{}
	closed    atomic.Bool
	connected atomic.Bool
	closeOnce sync.Once
}

func (b *BusImplNsqMQ) Healthy() bool {
	return b.connected.Load() && !b.closed.Load()
}

/**
* @Description: 创建nsq impl
* @param: selfBusId
* @param: onRecvMsg
* @param: conf
* @return: *BusImplNsqMQ
* @Author: Iori
* @Date: 2022-04-29 11:14:28
**/
func NewBusImplNsqMQ(selfBusId uint32, onRecvMsg bus.MsgHandler, conf bus.NSQConfig) *BusImplNsqMQ {
	impl := new(BusImplNsqMQ)

	impl.selfBusId = selfBusId
	impl.lookupAddr = conf.LookupAddrs
	impl.NsqdAddr = conf.NsqdAddr
	impl.chanName = conf.Channel
	impl.topicPrefix = conf.TopicPrefix
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan wire.OutMsg, 10000)
	impl.chanIn = make(chan []byte, 10000)
	impl.onRecv = onRecvMsg
	impl.concurrency = conf.Concurrency
	impl.maxInFlight = conf.MaxInFlight
	// 解析 lookupd_poll_interval，默认 3s
	impl.lookupdPollInterval = 3 * time.Second
	if conf.LookupdPollInterval != "" {
		if d, err := time.ParseDuration(conf.LookupdPollInterval); err == nil && d > 0 {
			impl.lookupdPollInterval = d
		}
	}
	impl.stopCh = make(chan struct{})

	go impl.run()
	return impl
}

/**
* @Description:
* @receiver: b
* @return: uint32
* @Author: Iori
* @Date: 2022-04-25 16:27:39
**/
func (b *BusImplNsqMQ) SelfBusId() uint32 {
	return b.selfBusId
}

/**
* @Description:
* @receiver: b
* @param: onRecvMsg
* @Author: Iori
* @Date: 2022-04-25 16:27:41
**/
func (b *BusImplNsqMQ) SetReceiver(onRecvMsg bus.MsgHandler) {
	b.onRecv = onRecvMsg
}

/**
* @Description: bus send
* @receiver: b
* @param: dstBusId
* @param: data1
* @param: data2
* @return: error
* @Author: Iori
* @Date: 2022-04-25 16:27:44
**/
func (b *BusImplNsqMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	msg := wire.OutMsg{
		BusID:  dstBusId,
		Topics: b.topicFor(dstBusId),
		Data:   wire.BuildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !wire.SendToMsgChan(b.chanOut, msg, b.timeout) {
		wire.PutFrameBuf(msg.Data)
		return fmt.Errorf("nsq bus.chanOut<-msg time out")
	} // msg所有权已转移，后面不能再使用msg
	return nil
}

func (b *BusImplNsqMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)
	})
	return nil
}

/**
* @Description:  normal send
* @receiver: b
* @param: topics
* @param: data1
* @param: data2
* @return: error
* @Author: Iori
* @Date: 2022-04-25 16:27:53
**/
func (b *BusImplNsqMQ) SendTo(topics string, data1 []byte, data2 []byte) error {
	msg := wire.OutMsg{}
	msg.Topics = topics
	msg.Data = make([]byte, len(data1)+len(data2))
	pos := 0
	copy(msg.Data[pos:], data1)
	pos += len(data1)
	if data2 != nil && len(data2) > 0 {
		copy(msg.Data[pos:], data2)
		pos += len(data2)
	}

	logger.Debugf("Send nsq bus message: %v \n", len(data1)+len(data2))
	if !wire.SendToMsgChan(b.chanOut, msg, b.timeout) {
		return fmt.Errorf("nsq bus.chanOut<-msg time out")
	}
	return nil
}

func (b *BusImplNsqMQ) topicFor(busId uint32) string {
	// Keep backward compatibility: if a topic prefix is provided, use "<prefix>_<hex>",
	// otherwise use wire.CalcQueueName(busId) which returns "bus_<hex>".
	if b.topicPrefix != "" {
		return fmt.Sprintf("%s_%x", b.topicPrefix, busId)
	}
	return wire.CalcQueueName(busId)
}

/**
* @Description: proc
* @receiver: b
* @return: error
* @Author: Iori
* @Date: 2022-04-25 16:28:12
**/
func (b *BusImplNsqMQ) process() error {
	//new Consumer
	consumer, err := nsqhelper.NewConsumerWithOpts(b.topicFor(b.selfBusId), b.chanName, b.NsqdAddr, b.lookupAddr, b.concurrency,
		b.maxInFlight, b.lookupdPollInterval,
		func(_ uint32, data []byte) error {
			// consumer callback may be concurrent; keep bus receiver single-thread by enqueueing only.
			if data == nil || len(data) == 0 {
				return nil
			}
			buf := make([]byte, len(data))
			copy(buf, data)
			select {
			case b.chanIn <- buf:
			case <-b.stopCh:
			}
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to open producer  {lookup: %v,addr:%v} | %v", b.lookupAddr, b.NsqdAddr, err)
	}
	defer consumer.Stop()
	logger.Infof("connected to %v", b.lookupAddr)

	//new Producer
	producer, err := nsqhelper.NewProducer(b.NsqdAddr)
	if err != nil {
		return fmt.Errorf("failed to open a producer  {addr:%v} | %v", b.NsqdAddr, err)
	}
	defer producer.Stop()

	b.connected.Store(true)
	defer b.connected.Store(false)

	//listen
	for {
		select {
		case <-b.stopCh:
			return nil
		case msgOut, ok := <-b.chanOut:
			if !ok {
				return fmt.Errorf("chanOut of bus is closed")
			}
			// send
			err = producer.Publish(msgOut.Topics, msgOut.Data)
			wire.PutFrameBuf(msgOut.Data) // Publish 同步等待响应，返回即可回收
			if err != nil {
				logger.Errorf("Failed to publish a message {topics:%v, dataLen:%v}| %v", msgOut.Topics, len(msgOut.Data), err)
				return err
			}
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
				// data 来自 chanIn，入队时已独立 make+copy（消费回调持有的 msg.Body
				// 生命周期不确定），取出后仅当前 goroutine 持有，可直接切片共享，
				// 无需再 copy 一份。上游 router.onRecvBusMsg 对 body 仅做切片引用。
				b.onRecv(header.SrcBusID, data[wire.HeaderLen():])
			}
		}
	}
}

/**
* @Description: run
* @receiver: b
* @Author: Iori
* @Date: 2022-04-25 16:28:16
**/
func (b *BusImplNsqMQ) run() {
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

		if time.Now().Sub(processStartTime) > time.Minute {
			retryCount = 0 // 正常运行1分钟以上，则重置retryCount
		}
		retryCount++
		retryAfterSeconds := (retryCount - 1) * 2
		if retryAfterSeconds > 30 {
			retryAfterSeconds = 30
		}
		logger.Errorf("Error occur in processing bus. Retry later {retryTimes: %v, afterSeconds:%v} | %v",
			retryCount, retryAfterSeconds, err)
		if !wire.SleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

// DriverName 是本 driver 注册时使用的 implType 字符串。
const DriverName = "nsq"

// newBus 是具体 ctor，同时被遗留的 func init 注册与显式 Driver() 描述符共用。
func newBus(selfBusId uint32, onRecvMsg bus.MsgHandler, conf any) (bus.IBus, error) {
	cfg, ok := conf.(bus.NSQConfig)
	if !ok {
		return nil, fmt.Errorf("nsq arg must be NSQConfig")
	}
	return NewBusImplNsqMQ(selfBusId, onRecvMsg, cfg), nil
}

// Driver 返回用于装配期注册的显式 driver 描述符。
func Driver() bus.Driver {
	return bus.Driver{Name: DriverName, Ctor: newBus}
}

func init() {
	bus.RegisterBus(DriverName, newBus)
}
