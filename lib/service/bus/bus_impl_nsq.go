package bus

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus/nsq"

	"time"
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

	timeout   time.Duration
	chanOut   chan outMsg
	chanIn    chan []byte
	onRecv    MsgHandler
	stopCh    chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
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
func NewBusImplNsqMQ(selfBusId uint32, onRecvMsg MsgHandler, conf NSQConfig) *BusImplNsqMQ {
	impl := new(BusImplNsqMQ)

	impl.selfBusId = selfBusId
	impl.lookupAddr = conf.LookupAddrs
	impl.NsqdAddr = conf.NsqdAddr
	impl.chanName = conf.Channel
	impl.topicPrefix = conf.TopicPrefix
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan outMsg, 10000)
	impl.chanIn = make(chan []byte, 10000)
	impl.onRecv = onRecvMsg
	impl.concurrency = conf.Concurrency
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
func (b *BusImplNsqMQ) SetReceiver(onRecvMsg MsgHandler) {
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
		return ErrBusClosed
	}
	header := busPacketHeader{}
	header.version = 0
	header.passCode = passCode
	header.srcBusId = b.SelfBusId()
	header.dstBusId = dstBusId

	msg := outMsg{}
	msg.busId = dstBusId
	msg.topics = b.topicFor(dstBusId)
	msg.data = make([]byte, byteLenOfBusPacketHeader()+len(data1)+len(data2))
	pos := 0
	header.To(msg.data[pos:])
	pos += byteLenOfBusPacketHeader()
	copy(msg.data[pos:], data1)
	pos += len(data1)
	if data2 != nil && len(data2) > 0 {
		copy(msg.data[pos:], data2)
		pos += len(data2)
	}

	logger.Debugf("Send nsq bus message: %v \n", len(data1)+len(data2))
	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
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
	msg := outMsg{}
	msg.topics = topics
	msg.data = make([]byte, len(data1)+len(data2))
	pos := 0
	copy(msg.data[pos:], data1)
	pos += len(data1)
	if data2 != nil && len(data2) > 0 {
		copy(msg.data[pos:], data2)
		pos += len(data2)
	}

	logger.Debugf("Send nsq bus message: %v \n", len(data1)+len(data2))
	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
		return fmt.Errorf("nsq bus.chanOut<-msg time out")
	}
	return nil
}

func (b *BusImplNsqMQ) topicFor(busId uint32) string {
	// Keep backward compatibility: if a topic prefix is provided, use "<prefix>_<hex>",
	// otherwise use calcQueueName(busId) which returns "bus_<hex>".
	if b.topicPrefix != "" {
		return fmt.Sprintf("%s_%x", b.topicPrefix, busId)
	}
	return calcQueueName(busId)
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
	consumer, err := nsq.NewConsumer(b.topicFor(b.selfBusId), b.chanName, b.NsqdAddr, b.lookupAddr, b.concurrency,
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
	producer, err := nsq.NewProducer(b.NsqdAddr)
	if err != nil {
		return fmt.Errorf("failed to open a producer  {addr:%v} | %v", b.NsqdAddr, err)
	}
	defer producer.Stop()

	//listen
	for {
		select {
		case <-b.stopCh:
			return nil
		case msgOut, ok := <-b.chanOut:
			if !ok {
				return fmt.Errorf("chanOut of bus is closed")
			}
			logger.Debugf("Send message to MQ: {dstBusId:0x%x, dataLen:%v}\n", msgOut.busId, len(msgOut.data))
			// send
			err = producer.Publish(msgOut.topics, msgOut.data)
			if err != nil {
				logger.Errorf("Failed to publish a message {topics:%v, dataLen:%v}| %v", msgOut.topics, len(msgOut.data), err)
				return err
			}
		case data, ok := <-b.chanIn:
			if !ok {
				return fmt.Errorf("chanIn of bus is closed")
			}
			if len(data) < byteLenOfBusPacketHeader() {
				continue
			}
			header := busPacketHeader{}
			header.From(data)
			if header.passCode != passCode {
				logger.Warningf("Received a bus message with wrong pass code: %#v", header)
				continue
			}
			if b.onRecv != nil {
				recvData := make([]byte, len(data)-byteLenOfBusPacketHeader())
				copy(recvData, data[byteLenOfBusPacketHeader():])
				b.onRecv(header.srcBusId, recvData)
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
		if !sleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

func init() {
	RegisterBus("nsq", func(selfBusId uint32, onRecvMsg MsgHandler, conf any) (IBus, error) {
		cfg, ok := conf.(NSQConfig)
		if !ok {
			return nil, fmt.Errorf("nsq arg must be NSQConfig")
		}
		return NewBusImplNsqMQ(selfBusId, onRecvMsg, cfg), nil
	})
}
