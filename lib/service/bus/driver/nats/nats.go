// Package nats provides the NATS bus driver.
// Import it (usually via driver/all) to enable "nats://" addresses.
package nats

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
	"github.com/nats-io/nats.go"
)

type BusImplNatsMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan wire.OutMsg
	chanIn    chan []byte
	onRecv    bus.MsgHandler

	url           string
	subjectPrefix string
	queueGroup    string
	stopCh        chan struct{}
	closed        atomic.Bool
	connected     atomic.Bool
	closeOnce     sync.Once
}

func NewBusImplNatsMQ(selfBusId uint32, onRecvMsg bus.MsgHandler, conf bus.NatsConfig) bus.IBus {
	prefix := strings.TrimSpace(conf.SubjectPrefix)
	if prefix == "" {
		prefix = "bus"
	}
	impl := &BusImplNatsMQ{
		selfBusId:     selfBusId,
		timeout:       3 * time.Second,
		chanOut:       make(chan wire.OutMsg, 10000),
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

func (b *BusImplNatsMQ) SelfBusId() uint32                    { return b.selfBusId }
func (b *BusImplNatsMQ) SetReceiver(onRecvMsg bus.MsgHandler) { b.onRecv = onRecvMsg }
func (b *BusImplNatsMQ) Healthy() bool                        { return b.connected.Load() && !b.closed.Load() }

// Start 等待后台 run goroutine 完成首次连接（IBus.Start 契约）。
func (b *BusImplNatsMQ) Start(ctx context.Context) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if b.Healthy() {
			return nil
		}
		if b.closed.Load() {
			return bus.ErrBusClosed
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("nats bus start: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (b *BusImplNatsMQ) subjectFor(busId uint32) string {
	return b.subjectPrefix + "." + wire.CalcQueueName(busId)
}

func (b *BusImplNatsMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	msg := wire.OutMsg{
		BusID:  dstBusId,
		Topics: b.subjectFor(dstBusId),
		Data:   wire.BuildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !wire.SendToMsgChan(b.chanOut, msg, b.timeout) {
		wire.PutFrameBuf(msg.Data)
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
			if err := nc.Publish(msgOut.Topics, msgOut.Data); err != nil {
				logger.Errorf("Failed to publish nats message {subject:%v, dataLen:%v}| %v", msgOut.Topics, len(msgOut.Data), err)
			}
			wire.PutFrameBuf(msgOut.Data) // Publish 已同步拷入客户端缓冲，可安全回收
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
				// data is our own copy made in the subscribe callback;
				// hand ownership to onRecv without a second copy.
				b.onRecv(header.SrcBusID, data[wire.HeaderLen():])
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
		if !wire.SleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

// DriverName 是本 driver 注册时使用的 implType 字符串。
const DriverName = "nats"

// newBus 是具体 ctor，同时被遗留的 func init 注册与显式 Driver() 描述符共用。
func newBus(selfBusId uint32, onRecvMsg bus.MsgHandler, conf any) (bus.IBus, error) {
	cfg, ok := conf.(bus.NatsConfig)
	if !ok {
		return nil, fmt.Errorf("nats arg must be NatsConfig")
	}
	return NewBusImplNatsMQ(selfBusId, onRecvMsg, cfg), nil
}

// Driver 返回用于装配期注册的显式 driver 描述符。
func Driver() bus.Driver {
	return bus.Driver{Name: DriverName, Ctor: newBus}
}
