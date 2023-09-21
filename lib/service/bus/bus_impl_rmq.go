package bus

import (
	"GoOne/lib/api/logger"
	"encoding/binary"
	"fmt"

	"github.com/streadway/amqp"

	"time"
)

type BusImplRabbitMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan outMsg
	onRecv    MsgHandler
}

func NewBusImplRabbitMQ(selfBusId uint32, onRecvMsg MsgHandler, rabbitmqAddr string) *BusImplRabbitMQ {
	impl := new(BusImplRabbitMQ)
	impl.selfBusId = selfBusId
	impl.timeout = 3 * time.Second
	impl.chanOut = make(chan outMsg, 10000)
	impl.onRecv = onRecvMsg
	go impl.run(rabbitmqAddr)
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

	logger.Debugf("Send bus message: %v, %#v\n", len(data1)+len(data2), header)

	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
		return fmt.Errorf("bus.chanOut<-msg time out")
	} // msg所有权已转移，后面不能再使用msg

	return nil
}

// -------------------------------- private --------------------------------

const (
	passCode = 0xFEED
)

type busPacket struct {
	Header busPacketHeader
	Body   []byte
}

type busPacketHeader struct {
	version  uint16
	passCode uint16
	srcBusId uint32
	dstBusId uint32
}

func byteLenOfBusPacketHeader() int {
	return 12
}

func (h *busPacketHeader) From(b []byte) {
	h.version = binary.BigEndian.Uint16(b[0:])
	h.passCode = binary.BigEndian.Uint16(b[2:])
	h.srcBusId = binary.BigEndian.Uint32(b[4:])
	h.dstBusId = binary.BigEndian.Uint32(b[8:])
}

func (h *busPacketHeader) To(b []byte) {
	binary.BigEndian.PutUint16(b[0:], h.version)
	binary.BigEndian.PutUint16(b[2:], h.passCode)
	binary.BigEndian.PutUint32(b[4:], h.srcBusId)
	binary.BigEndian.PutUint32(b[8:], h.dstBusId)
}

type outMsg struct {
	busId uint32
	data  []byte
}

func calcQueueName(busId uint32) string {
	return "bus_" + fmt.Sprintf("%x", busId)
}

func sendToMsgChan(ch chan outMsg, msg outMsg, timeout time.Duration) bool {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case ch <- msg:
	case <-t.C:
		return false
	}

	return true
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

			logger.Debugf("Send message to MQ: {dstBusId:0x%x, dataLen:%v}\n", msgOut.busId, len(msgOut.data))
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
				logger.Errorf("Failed to publish a message {busId:%v, dataLen:%v}| %w",
					msgOut.busId, len(msgOut.data), err)
				// todo: is it necessary to return the err?
			}
		case delivery, ok := <-chanRecv:
			if !ok {
				return fmt.Errorf("chanRecv of bus is closed")
			}

			header := busPacketHeader{}
			header.From(delivery.Body)
			logger.Debugf("Received message from MQ: %#v\n", header)
			if header.passCode != passCode {
				logger.Warningf("Received a bus message with wrong pass code: %#v\n", header)
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
		logger.Errorf("Error occur in processing bus. Retry later {retryTimes: %v, afterSeconds:%v} | %w",
			retryCount, retryAfterSeconds, err)
		time.Sleep(time.Duration(retryAfterSeconds) * time.Second)
	}
}
