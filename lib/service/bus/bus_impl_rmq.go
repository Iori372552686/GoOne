package bus

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"

	"github.com/streadway/amqp"

	"time"
)

type BusImplRabbitMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan outMsg
	onRecv    MsgHandler
}

func NewBusImplRabbitMQ(selfBusId uint32, onRecvMsg MsgHandler, addr string) *BusImplRabbitMQ {
	impl := new(BusImplRabbitMQ)
	impl.selfBusId = selfBusId
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan outMsg, 10000)
	impl.onRecv = onRecvMsg
	go impl.run(addr)
	return impl
}

func (b *BusImplRabbitMQ) SelfBusId() uint32 {
	return b.selfBusId
}

func (b *BusImplRabbitMQ) SetReceiver(onRecvMsg MsgHandler) {
	b.onRecv = onRecvMsg
}

func (b *BusImplRabbitMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
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

	//logger.Debugf("Send bus message: %v, %#v\n", len(data1)+len(data2), header)
	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
		return fmt.Errorf("bus.chanOut<-msg time out")
	} // msg所有权已转移，后面不能再使用msg

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

	for {
		select {
		case msgOut, ok := <-b.chanOut:
			if !ok {
				return fmt.Errorf("chanOut of bus is closed")
			}

			// send by routering
			err = ch.Publish(
				"",                          // exchange
				calcQueueName(msgOut.busId), // routing key
				false,                       // mandatory
				false,                       // immediate
				amqp.Publishing{
					// ContentType: "text/plain",
					Body: msgOut.data,
				})
			if err != nil {
				logger.Errorf("Failed to publish a message {busId:%v, dataLen:%v}| %v", msgOut.busId, len(msgOut.data), err)
				// todo: is it necessary to return the err?
			}
		case delivery, ok := <-chanRecv:
			if !ok {
				return fmt.Errorf("chanRecv of bus is closed")
			}

			header := busPacketHeader{}
			header.From(delivery.Body)
			//logger.Debugf("Received message from MQ: %+v", header)
			if header.passCode != passCode {
				logger.Warningf("Received a bus message with wrong pass code: %#v", header)
				break
			}

			if b.onRecv != nil {
				recvData := make([]byte, len(delivery.Body)-byteLenOfBusPacketHeader())
				copy(recvData, delivery.Body[byteLenOfBusPacketHeader():])
				// Todo:不确定delivery.Body的生命周期，保险起见，这里还是先拷贝了一份。
				b.onRecv(header.srcBusId, recvData)
			}
		}
	}

	return nil
}

func (b *BusImplRabbitMQ) run(rabbitmqAddr string) {
	myQueueName := calcQueueName(b.selfBusId)
	logger.Errorf("Start bus service {myQueueName:%s}", myQueueName)

	retryCount := 0
	for {
		processStartTime := time.Now()

		err := b.process(rabbitmqAddr, myQueueName)

		if time.Now().Sub(processStartTime) > time.Minute {
			retryCount = 0 // 正常运行1分钟以上，则重置retryCount
		}
		retryCount++
		retryAfterSeconds := (retryCount - 1) * 2
		if retryAfterSeconds > 30 {
			retryAfterSeconds = 30
		}
		logger.Errorf("Error occur in processing bus. Retry later {retryTimes: %v, afterSeconds:%v} | %v", retryCount, retryAfterSeconds, err)
		time.Sleep(time.Duration(retryAfterSeconds) * time.Second)
	}
}
