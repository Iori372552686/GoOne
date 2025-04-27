package bus

import (
	"fmt"
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
	topics      string
	chanName    string
	concurrency int

	timeout time.Duration
	chanOut chan outMsg
	onRecv  MsgHandler
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
func NewBusImplNsqMQ(selfBusId uint32, onRecvMsg MsgHandler, conf Config) *BusImplNsqMQ {
	impl := new(BusImplNsqMQ)

	impl.selfBusId = selfBusId
	impl.lookupAddr = conf.LookupAddrs
	impl.NsqdAddr = fmt.Sprintf("%s:%d", conf.IPAddr, conf.Port)
	impl.chanName = conf.ChanName
	impl.topics = conf.Topics
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan outMsg, 10000)
	impl.onRecv = onRecvMsg
	impl.concurrency = conf.Concurrency

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
	header := busPacketHeader{}
	header.version = 0
	header.passCode = passCode
	header.srcBusId = b.SelfBusId()
	header.dstBusId = dstBusId

	msg := outMsg{}
	msg.busId = dstBusId
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

/**
* @Description: proc
* @receiver: b
* @return: error
* @Author: Iori
* @Date: 2022-04-25 16:28:12
**/
func (b *BusImplNsqMQ) process() error {
	//new Consumer
	consumer, err := nsq.NewConsumer(b.topics, b.chanName, b.NsqdAddr, b.lookupAddr, b.concurrency, nsq.MsgHandler(b.onRecv))
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
		}
	}

	return nil
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
		processStartTime := time.Now()

		err := b.process()

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
		time.Sleep(time.Duration(retryAfterSeconds) * time.Second)
	}
}
