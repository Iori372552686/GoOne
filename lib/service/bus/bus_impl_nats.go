package bus

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nats-io/nats.go"
)

type BusImplNatsMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan outMsg
	chanIn    chan []byte
	onRecv    MsgHandler

	url           string
	subjectPrefix string
	queueGroup    string
	stopCh        chan struct{}
	closed        atomic.Bool
	connected     atomic.Bool
	closeOnce     sync.Once
}

func NewBusImplNatsMQ(selfBusId uint32, onRecvMsg MsgHandler, conf NatsConfig) IBus {
	prefix := strings.TrimSpace(conf.SubjectPrefix)
	if prefix == "" {
		prefix = "bus"
	}
	impl := &BusImplNatsMQ{
		selfBusId:     selfBusId,
		timeout:       3 * time.Second,
		chanOut:       make(chan outMsg, 10000),
		chanIn:        make(chan []byte, 10000),
		onRecv:        onRecvMsg,
		url:           strings.TrimSpace(conf.URL),
		subjectPrefix: prefix,
		queueGroup:    strings.TrimSpace(conf.QueueGroup),
		stopCh:        make(chan struct{}),
	}
	go impl.run()
	return impl
}

func (b *BusImplNatsMQ) SelfBusId() uint32                { return b.selfBusId }
func (b *BusImplNatsMQ) SetReceiver(onRecvMsg MsgHandler) { b.onRecv = onRecvMsg }
func (b *BusImplNatsMQ) Healthy() bool                    { return b.connected.Load() && !b.closed.Load() }

func (b *BusImplNatsMQ) subjectFor(busId uint32) string {
	return b.subjectPrefix + "." + calcQueueName(busId)
}

func (b *BusImplNatsMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	msg := outMsg{
		busId:  dstBusId,
		topics: b.subjectFor(dstBusId),
		data:   buildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
		putFrameBuf(msg.data)
		return fmt.Errorf("nats bus.chanOut<-msg time out")
	}
	return nil
}

func (b *BusImplNatsMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)
	})
	return nil
}

func (b *BusImplNatsMQ) process() error {
	if b.url == "" {
		return fmt.Errorf("nats url is empty")
	}
	nc, err := nats.Connect(
		b.url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return err
	}
	defer nc.Close()

	mySubject := b.subjectFor(b.selfBusId)
	logger.Infof("NATS bus connected, subscribe: %s", mySubject)

	// handleMsg copies the payload (nats owns m.Data) and enqueues it with a
	// bounded wait, so a stuck consumer can never block the NATS callback
	// thread forever.
	handleMsg := func(m *nats.Msg) {
		buf := make([]byte, len(m.Data))
		copy(buf, m.Data)

		select {
		case b.chanIn <- buf:
			return
		default:
		}

		t := time.NewTimer(3 * time.Second)
		defer t.Stop()
		select {
		case b.chanIn <- buf:
		case <-b.stopCh:
		case <-t.C:
			logger.Errorf("nats bus chanIn full, drop message {len:%d}", len(buf))
		}
	}

	var sub *nats.Subscription
	if b.queueGroup != "" {
		sub, err = nc.QueueSubscribe(mySubject, b.queueGroup, handleMsg)
	} else {
		sub, err = nc.Subscribe(mySubject, handleMsg)
	}
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	_ = nc.Flush()

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
			if err := nc.Publish(msgOut.topics, msgOut.data); err != nil {
				logger.Errorf("Failed to publish nats message {subject:%v, dataLen:%v}| %v", msgOut.topics, len(msgOut.data), err)
			}
			putFrameBuf(msgOut.data) // Publish 已同步拷入客户端缓冲，可安全回收
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
				// data is our own copy made in the subscribe callback;
				// hand ownership to onRecv without a second copy.
				b.onRecv(header.srcBusId, data[byteLenOfBusPacketHeader():])
			}
		}
	}
}

func (b *BusImplNatsMQ) run() {
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
		logger.Errorf("Error occur in processing bus(nats). Retry later {retryTimes: %v, afterSeconds:%v} | %v",
			retryCount, retryAfterSeconds, err)
		if !sleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

func init() {
	RegisterBus("nats", func(selfBusId uint32, onRecvMsg MsgHandler, conf any) (IBus, error) {
		cfg, ok := conf.(NatsConfig)
		if !ok {
			return nil, fmt.Errorf("nats arg must be NatsConfig")
		}
		return NewBusImplNatsMQ(selfBusId, onRecvMsg, cfg), nil
	})
}
