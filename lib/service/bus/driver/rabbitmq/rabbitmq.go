// Package rabbitmq provides the RabbitMQ bus driver.
// Import it (usually via driver/all) to enable "amqp://" / "rabbitmq://" addresses.
package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/internal/wire"
	amqp "github.com/rabbitmq/amqp091-go"
)

// DefaultStartTimeout 是 Start 等待首次连接的兜底上限（V4 P0-07：Start 不再
// fire-and-forget）。当调用方未在 context 上设置 deadline 时采用。
const DefaultStartTimeout = 10 * time.Second

// BusImplRabbitMQ 实现 bus.IBus 与可选的 bus.Starter / RuntimeErrorSource。
//
// V4 P0-07 生命周期契约：
//   - NewBusImplRabbitMQ 不启动后台 goroutine、不连接；Start 同步完成
//     「Dial → Channel → QueueDeclare → Consume」，返回 nil 即表示 RabbitMQ 已连接、
//     队列已声明、consumer 已注册，并已启动消费+重连 goroutine；
//   - Send 为同步 Publish：调用方拿到 nil 即表示消息已被当前连接成功 publish，
//     publish 失败直接回传 error，绝不「记日志后返回 nil」产生虚假成功；
//   - Close 关闭 stopCh 并 WaitGroup join 全部 goroutine，幂等；
//   - RuntimeErrors 在运行期连接断开时投递非 nil error，供 Router 注入 runtime
//     error，触发标准 Drain/Failed（readyz 自动摘流）。
type BusImplRabbitMQ struct {
	selfBusId uint32
	addr      string
	queueArgs amqp.Table

	mu         sync.Mutex // 保护 conn/channel/deliveries/onRecv 与 started 状态
	onRecv     bus.MsgHandler
	conn       *amqp.Connection
	channel    *amqp.Channel
	deliveries <-chan amqp.Delivery
	started    bool

	closed    atomic.Bool
	connected atomic.Bool
	closeOnce sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup // join run() goroutine

	pubMu sync.Mutex // 串行化当前 channel 上的 Publish（amqp091 channel 非并发安全）

	runtimeErrors chan error // 容量 1：断线事件在订阅前发生也不丢失
}

// NewBusImplRabbitMQ 构造实例但不连接（V4 P0-07：连接延迟到 Start）。
func NewBusImplRabbitMQ(selfBusId uint32, onRecvMsg bus.MsgHandler, addr string) *BusImplRabbitMQ {
	return &BusImplRabbitMQ{
		selfBusId: selfBusId,
		addr:      addr,
		queueArgs: amqp.Table{
			"x-message-ttl":      int32(30 * 60 * 1000),
			"x-max-length-bytes": int32(10 * 1024 * 1024),
			"x-overflow":         "reject-publish",
		},
		onRecv:        onRecvMsg,
		stopCh:        make(chan struct{}),
		runtimeErrors: make(chan error, 1),
	}
}

func (b *BusImplRabbitMQ) SelfBusId() uint32 { return b.selfBusId }

func (b *BusImplRabbitMQ) SetReceiver(onRecvMsg bus.MsgHandler) {
	b.mu.Lock()
	b.onRecv = onRecvMsg
	b.mu.Unlock()
}

// Healthy 反映「已连接且未关闭」。Start 成功后为 true；断线重连期间为 false。
func (b *BusImplRabbitMQ) Healthy() bool {
	return b.connected.Load() && !b.closed.Load()
}

// Start 同步完成首次连接与 consumer 创建（V4 P0-07）。
//
// 契约：返回 nil 表示 RabbitMQ 已连接、队列已声明、consumer 已注册，且消费/重连
// goroutine 已启动。任一阶段失败按 ctx 返回 error，不启动后台 goroutine、不泄漏连接。
// 已 Start 或已 Close 时返回 error。
func (b *BusImplRabbitMQ) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.started {
		b.mu.Unlock()
		return errors.New("rabbitmq bus already started")
	}
	if b.closed.Load() {
		b.mu.Unlock()
		return bus.ErrBusClosed
	}
	b.mu.Unlock()

	if err := b.connectOnce(ctx); err != nil {
		return fmt.Errorf("rabbitmq start: %w", err)
	}

	b.mu.Lock()
	if b.closed.Load() {
		// 在首次连接期间被 Close：清理并返回。
		ch, conn := b.channel, b.conn
		b.channel, b.conn, b.deliveries = nil, nil, nil
		b.mu.Unlock()
		_ = ch.Close()
		_ = conn.Close()
		return bus.ErrBusClosed
	}
	b.started = true
	b.mu.Unlock()

	b.wg.Add(1)
	go b.run()
	return nil
}

// connectOnce 执行 Dial→Channel→QueueDeclare→Consume，把就绪的 conn/channel/
// deliveries 存入字段并置 connected=true。失败时清理已创建资源，返回 error。
func (b *BusImplRabbitMQ) connectOnce(ctx context.Context) error {
	conn, ch, deliveries, err := b.setup(ctx)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.conn = conn
	b.channel = ch
	b.deliveries = deliveries
	b.connected.Store(true)
	b.mu.Unlock()
	queueName := wire.CalcQueueName(b.selfBusId)
	logger.Infof("rabbitmq connected {addr:%s, queue:%s}", b.redactedAddr(), queueName)
	return nil
}

// setup 执行 Dial→Channel→QueueDeclare→Qos→Consume，返回就绪资源。失败时清理。
func (b *BusImplRabbitMQ) setup(ctx context.Context) (*amqp.Connection, *amqp.Channel, <-chan amqp.Delivery, error) {
	conn, err := amqp.Dial(b.addr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("dial %s: %w", b.redactedAddr(), err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("open channel: %w", err)
	}
	queueName := wire.CalcQueueName(b.selfBusId)
	if _, err = ch.QueueDeclare(queueName, false, false, false, false, b.queueArgs); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("declare queue %s: %w", queueName, err)
	}
	if err = ch.Qos(64, 0, false); err != nil {
		logger.Warningf("rabbitmq ch.Qos failed: %v", err) // 非致命
	}
	deliveries, err := ch.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, nil, fmt.Errorf("register consumer: %w", err)
	}
	return conn, ch, deliveries, nil
}

// run 是唯一的消费+重连 goroutine（V4 P0-07）。
//
// 阻塞在当前 channel 的 deliveries / close 通知上分发消息；channel 关闭（断线）时
// 标记未就绪、上报 runtime error，按可取消退避重连（重建 conn/channel/deliveries
// 三者一起替换）。Stop 信号使其返回，由 Close 经 WaitGroup join。
func (b *BusImplRabbitMQ) run() {
	defer b.wg.Done()
	defer b.connected.Store(false)

	retry := 0
	for {
		if b.closed.Load() {
			return
		}

		b.mu.Lock()
		deliveries := b.deliveries
		ch := b.channel
		b.mu.Unlock()

		if deliveries == nil || ch == nil {
			// 无可用 channel（首次已由 Start 建立；此处仅重连路径会重建）。
			if !b.reconnect(ctxFromStop(b.stopCh), &retry) {
				return
			}
			continue
		}

		closed := ch.NotifyClose(make(chan *amqp.Error, 1))
		disconnected := b.consume(deliveries, closed)

		if b.closed.Load() {
			return
		}
		b.connected.Store(false)
		b.reportRuntimeError(fmt.Errorf("router.rabbitmq.consumer: %v", errChannelDisconnected))
		if !disconnected {
			return
		}
		// 重连前关闭残留 channel/conn，避免句柄泄漏。
		b.mu.Lock()
		oldCh, oldConn := b.channel, b.conn
		b.channel, b.conn, b.deliveries = nil, nil, nil
		b.mu.Unlock()
		if oldCh != nil {
			_ = oldCh.Close()
		}
		if oldConn != nil {
			_ = oldConn.Close()
		}
		if !b.reconnect(ctxFromStop(b.stopCh), &retry) {
			return
		}
	}
}

var errChannelDisconnected = errors.New("channel disconnected")

// consume 在当前 deliveries 流上分发消息，直到 channel 关闭或 Stop。
// 返回 true 表示因 channel 关闭退出（应重连），false 表示因 Stop 退出。
func (b *BusImplRabbitMQ) consume(deliveries <-chan amqp.Delivery, closed chan *amqp.Error) bool {
	for {
		select {
		case <-b.stopCh:
			return false
		case amqpErr, ok := <-closed:
			if ok && amqpErr != nil {
				logger.Errorf("rabbitmq channel closed: %v", amqpErr)
			}
			return true
		case d, ok := <-deliveries:
			if !ok {
				logger.Warningf("rabbitmq deliveries channel closed")
				return true
			}
			if len(d.Body) < wire.HeaderLen() {
				logger.Warningf("rabbitmq message too short {len:%v, expect:%v}", len(d.Body), wire.HeaderLen())
				continue
			}
			header := wire.Header{}
			header.From(d.Body)
			if header.PassCode != wire.PassCode {
				logger.Warningf("rabbitmq wrong pass code: %#v", header)
				continue
			}
			b.mu.Lock()
			onRecv := b.onRecv
			b.mu.Unlock()
			if onRecv != nil {
				// amqp091 delivery.Body 在 dispatch 后可安全持有（见历史注释），
				// 上游 router.onRecvBusMsg 仅做切片引用，无需防御性 copy。
				if err := onRecv(header.SrcBusID, d.Body[wire.HeaderLen():]); err != nil {
					logger.Errorf("rabbitmq onRecv error: %v", err)
				}
			}
		}
	}
}

// reconnect 按可取消退避重连，成功后重置 retry 计数。Stop 期间返回 false。
func (b *BusImplRabbitMQ) reconnect(ctx context.Context, retry *int) bool {
	for {
		if b.closed.Load() {
			return false
		}
		*retry++
		backoff := time.Duration(*retry-1) * 2 * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		if backoff > 0 {
			logger.Errorf("rabbitmq reconnect retry {times:%d, after:%v}", *retry, backoff)
			if !wire.SleepOrStop(b.stopCh, backoff) {
				return false
			}
		}
		if err := b.connectOnce(ctx); err != nil {
			logger.Errorf("rabbitmq reconnect failed {addr:%s}: %v", b.redactedAddr(), err)
			continue
		}
		*retry = 0
		return true
	}
}

// Send 同步 publish（V4 P0-07：成功表示已被当前连接成功 publish，失败回传 error）。
//
//   - closed 时立即返回 ErrBusClosed；
//   - 非就绪时返回带明确语义的 error（调用方可重试或降级）；
//   - 在 pubMu 保护下同步 Publish，序列化到当前 channel 发送缓冲后再返回；
//   - publish 返回 error 时回传，绝不「记日志后返回 nil」。
func (b *BusImplRabbitMQ) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	if b.closed.Load() {
		return bus.ErrBusClosed
	}
	if !b.connected.Load() {
		return fmt.Errorf("rabbitmq bus not connected")
	}

	frame := wire.BuildFrame(b.SelfBusId(), dstBusId, data1, data2)
	defer wire.PutFrameBuf(frame)

	b.mu.Lock()
	ch := b.channel
	b.mu.Unlock()
	if ch == nil {
		return fmt.Errorf("rabbitmq channel unavailable")
	}

	b.pubMu.Lock()
	defer b.pubMu.Unlock()
	if err := ch.Publish(
		"",                           // exchange
		wire.CalcQueueName(dstBusId), // routing key
		false,                        // mandatory
		false,                        // immediate
		amqp.Publishing{Body: frame},
	); err != nil {
		return fmt.Errorf("publish {dst:%#x, len:%d}: %w", dstBusId, len(frame), err)
	}
	return nil
}

// Close 关闭连接、取消所有 goroutine 并 join（V4 P0-07）。幂等。
func (b *BusImplRabbitMQ) Close() error {
	b.closeOnce.Do(func() {
		b.closed.Store(true)
		b.connected.Store(false)
		close(b.stopCh)
		b.wg.Wait()

		b.mu.Lock()
		ch, conn := b.channel, b.conn
		b.channel, b.conn, b.deliveries = nil, nil, nil
		b.mu.Unlock()
		if ch != nil {
			_ = ch.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
	})
	return nil
}

// RuntimeErrors 实现 RuntimeErrorSource：运行期连接断开时投递非 nil error
// （V4 P0-07：Bus 运行期失败进入 RuntimeErrors，触发标准 Drain/Failed）。
func (b *BusImplRabbitMQ) RuntimeErrors() <-chan error {
	return b.runtimeErrors
}

func (b *BusImplRabbitMQ) reportRuntimeError(err error) {
	select {
	case b.runtimeErrors <- err:
	default:
		// 容量 1，已有未消费事件则丢弃旧的，保留最新一次断线原因。
		select {
		case <-b.runtimeErrors:
		default:
		}
		select {
		case b.runtimeErrors <- err:
		default:
		}
	}
}

// redactedAddr 返回去掉用户名密码的 host:port，供日志使用（V4 P0-07：地址脱敏）。
func (b *BusImplRabbitMQ) redactedAddr() string {
	return redactAMQPAddr(b.addr)
}

// ctxFromStop 把 stopCh 适配为可取消的 context（用于 setup 的语义统一）。
func ctxFromStop(stopCh <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

// redactAMQPAddr 把 amqp://user:pass@host:port/v 中的凭据抹去，返回 host:port。
func redactAMQPAddr(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return "<unparseable>"
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5672"
	}
	if host == "" {
		return "<no-host>"
	}
	return host + ":" + port
}

// DriverName 是本 driver 注册时使用的 implType 字符串。
const DriverName = "rabbitmq"

// newBus 是具体 ctor，同时被遗留的 func init 注册与显式 Driver() 描述符共用，使两
// 条路径不会分叉。
func newBus(selfBusId uint32, onRecvMsg bus.MsgHandler, conf any) (bus.IBus, error) {
	var addr string
	switch v := conf.(type) {
	case string:
		addr = v
	case bus.RabbitMQConfig:
		addr = v.Addr
	default:
		return nil, fmt.Errorf("rabbitmq unsupported config type %T", conf)
	}
	if addr == "" {
		return nil, fmt.Errorf("rabbitmq addr is empty")
	}
	return NewBusImplRabbitMQ(selfBusId, onRecvMsg, addr), nil
}

// Driver 返回用于装配期注册的显式 driver 描述符。只想链接本 driver 的应用，应通过
// bus.DriverRegistry.MustRegister(rabbitmq.Driver()) 注册，而非 blank-import
// driver/all。
func Driver() bus.Driver {
	return bus.Driver{Name: DriverName, Ctor: newBus}
}
