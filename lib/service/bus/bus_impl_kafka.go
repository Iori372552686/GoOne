package bus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/segmentio/kafka-go"
)

type BusImplKafkaMQ struct {
	selfBusId uint32
	timeout   time.Duration
	chanOut   chan outMsg
	chanIn    chan []byte
	onRecv    MsgHandler

	brokers       []string
	topicPrefix   string
	groupIDPrefix string
	stopCh        chan struct{}
	closed        atomic.Bool
	connected     atomic.Bool
	closeOnce     sync.Once
}

func NewBusImplKafkaMQ(selfBusId uint32, onRecvMsg MsgHandler, conf KafkaConfig) IBus {
	topicPrefix := strings.TrimSpace(conf.TopicPrefix)
	if topicPrefix == "" {
		topicPrefix = "bus"
	}
	groupPrefix := strings.TrimSpace(conf.GroupIDPrefix)
	if groupPrefix == "" {
		groupPrefix = "bus"
	}
	impl := &BusImplKafkaMQ{
		selfBusId:     selfBusId,
		timeout:       3 * time.Second,
		chanOut:       make(chan outMsg, 10000),
		chanIn:        make(chan []byte, 10000),
		onRecv:        onRecvMsg,
		brokers:       conf.Brokers,
		topicPrefix:   topicPrefix,
		groupIDPrefix: groupPrefix,
		stopCh:        make(chan struct{}),
	}
	go impl.run()
	return impl
}

func (b *BusImplKafkaMQ) SelfBusId() uint32                { return b.selfBusId }
func (b *BusImplKafkaMQ) SetReceiver(onRecvMsg MsgHandler) { b.onRecv = onRecvMsg }
func (b *BusImplKafkaMQ) Healthy() bool                    { return b.connected.Load() && !b.closed.Load() }

func (b *BusImplKafkaMQ) topicFor(busId uint32) string {
	return b.topicPrefix + "." + calcQueueName(busId)
}

func (b *BusImplKafkaMQ) groupFor(busId uint32) string {
	return b.groupIDPrefix + "." + calcQueueName(busId)
}

func (b *BusImplKafkaMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	msg := outMsg{
		busId:  dstBusId,
		topics: b.topicFor(dstBusId),
		data:   buildFrame(b.SelfBusId(), dstBusId, data1, data2),
	}

	if !sendToMsgChan(b.chanOut, msg, b.timeout) {
		putFrameBuf(msg.data)
		return fmt.Errorf("kafka bus.chanOut<-msg time out")
	}
	return nil
}

func (b *BusImplKafkaMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		close(b.stopCh)
	})
	return nil
}

func (b *BusImplKafkaMQ) process() error {
	if len(b.brokers) == 0 {
		return fmt.Errorf("kafka brokers is empty")
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(b.brokers...),
		RequiredAcks: kafka.RequireOne,
		Balancer:     &kafka.LeastBytes{},
		Async:        false,
	}
	defer w.Close()

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  b.brokers,
		Topic:    b.topicFor(b.selfBusId),
		GroupID:  b.groupFor(b.selfBusId),
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer r.Close()

	ctx := context.Background()

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	// readErr propagates reader failures so the retry loop reconnects instead
	// of silently losing the consumer while the writer keeps running.
	readErr := make(chan error, 1)
	go func() {
		for {
			m, err := r.ReadMessage(readCtx)
			if err != nil {
				select {
				case readErr <- err:
				default:
				}
				return
			}
			// m.Value is freshly allocated by kafka-go per message; pass
			// ownership downstream without an extra copy.
			select {
			case b.chanIn <- m.Value:
			case <-b.stopCh:
				return
			}
		}
	}()

	b.connected.Store(true)
	defer b.connected.Store(false)

	for {
		select {
		case <-b.stopCh:
			return nil
		case err := <-readErr:
			return fmt.Errorf("kafka reader failed: %w", err)
		case msgOut, ok := <-b.chanOut:
			if !ok {
				return fmt.Errorf("chanOut of bus is closed")
			}
			if err := w.WriteMessages(ctx, kafka.Message{
				Topic: msgOut.topics,
				Value: msgOut.data,
			}); err != nil {
				logger.Errorf("Failed to publish kafka message {topic:%v, dataLen:%v}| %v", msgOut.topics, len(msgOut.data), err)
			}
			putFrameBuf(msgOut.data) // 同步 writer（Async:false），返回即写完，可安全回收
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
				// data ownership is ours (allocated by the reader goroutine);
				// hand it to onRecv without a second copy.
				b.onRecv(header.srcBusId, data[byteLenOfBusPacketHeader():])
			}
		}
	}
}

func (b *BusImplKafkaMQ) run() {
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
		logger.Errorf("Error occur in processing bus(kafka). Retry later {retryTimes: %v, afterSeconds:%v} | %v",
			retryCount, retryAfterSeconds, err)
		if !sleepOrStop(b.stopCh, time.Duration(retryAfterSeconds)*time.Second) {
			return
		}
	}
}

func init() {
	RegisterBus("kafka", func(selfBusId uint32, onRecvMsg MsgHandler, conf any) (IBus, error) {
		cfg, ok := conf.(KafkaConfig)
		if !ok {
			return nil, fmt.Errorf("kafka arg must be KafkaConfig")
		}
		return NewBusImplKafkaMQ(selfBusId, onRecvMsg, cfg), nil
	})
}
