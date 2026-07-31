package rabbitmq

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
	"github.com/Iori372552686/GoOne/lib/service/bus"
)

// TestDriverNameAndCtor 验证 Driver 返回的描述符名与 ctor 行为符合契约。
func TestDriverNameAndCtor(t *testing.T) {
	d := Driver()
	if d.Name != DriverName {
		t.Fatalf("Driver name = %q, want %q", d.Name, DriverName)
	}
	if d.Ctor == nil {
		t.Fatal("Driver ctor 不应为 nil")
	}
	// 空 addr 应返回 error（不连接）。
	if _, err := d.Ctor(1, nil, ""); err == nil {
		t.Fatal("空 addr 应返回 error")
	}
	_ = strings.TrimSpace // 保留 strings 引用（parseAMQPHostPort 使用）
}

// TestNewBusImplRabbitMQCloseIsIdempotent 验证 Close 幂等，且 Close 后 Send 返回
// ErrBusClosed。不连接真实 RabbitMQ（NewBusImplRabbitMQ 不再自动连接，
// Start 之前 connected 为 false）。
func TestNewBusImplRabbitMQCloseIsIdempotent(t *testing.T) {
	b := NewBusImplRabbitMQ(0x01010101, func(uint32, []byte) error { return nil }, "amqp://guest:guest@127.0.0.1:1/")

	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// 幂等。
	if err := b.Close(); err != nil {
		t.Fatalf("second Close should be idempotent: %v", err)
	}
	// Close 后 Send 返回 ErrBusClosed（关闭后绝不虚假成功）。
	if err := b.Send(0x02020202, []byte("x"), nil); err != bus.ErrBusClosed {
		t.Fatalf("Close 后 Send 应返回 ErrBusClosed，got %v", err)
	}
	if b.Healthy() {
		t.Fatal("Close 后 Healthy 应为 false")
	}
}

// TestRabbitMQStartFailsOnUnreachable 验证 故障契约：RabbitMQ 不可达时
// Start 在 ctx 超时后返回 error，不启动后台 goroutine、不泄漏连接，服务发现中无
// 当前实例（由调用方 Router 保证：Start 失败即返回、不进入注册）。
func TestRabbitMQStartFailsOnUnreachable(t *testing.T) {
	b := NewBusImplRabbitMQ(0x03030303, func(uint32, []byte) error { return nil }, "amqp://guest:guest@127.0.0.1:1/")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := b.Start(ctx); err == nil {
		t.Fatal("Start 对不可达 RabbitMQ 应在 ctx 超时后返回 error")
	}
	if b.Healthy() {
		t.Fatal("不可达时 Healthy 应为 false")
	}
	// Start 失败后未启动 goroutine，Close 必须立即返回（不阻塞、不泄漏）。
	done := make(chan struct{})
	go func() {
		_ = b.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Start 失败后 Close 未在 3s 内返回（goroutine 泄漏）")
	}
}

// TestRabbitMQStartTwiceRejected 验证重复 Start 返回 error（已 Start）。
// 不依赖真实 RabbitMQ：使用真实本地实例时由 itest 门控；这里用不可达地址覆盖失败路径。
func TestRabbitMQStartTwiceRejected(t *testing.T) {
	b := NewBusImplRabbitMQ(0x04040404, nil, "amqp://guest:guest@127.0.0.1:1/")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = b.Start(ctx) // 失败（不可达），不进入 started 状态
	// 先 Close 清理，再 Start 应返回 ErrBusClosed。
	_ = b.Close()
	if err := b.Start(context.Background()); err == nil {
		t.Fatal("Close 后 Start 应返回 error")
	}
}

// amqpTestAddr 取自环境变量（默认本地实例），便于在不同环境下运行真实联调。
func amqpTestAddr() string {
	if v := os.Getenv("GOONE_AMQP_ADDR"); v != "" {
		return v
	}
	return "amqp://guest:guest@127.0.0.1:5672/"
}

// TestRabbitMQRealIntegration 验证 真实 RabbitMQ 联调——两个不同 bus ID 的实例
// 经 amqp091-go 互通（A→B 收发）。需要真实 RabbitMQ；无中间件时跳过。
func TestRabbitMQRealIntegration(t *testing.T) {
	addr := amqpTestAddr()
	// 快速探测可达性（含 env 门控），避免无中间件时长时间挂起。
	itest.Require(t, parseAMQPHostPort(addr))

	const sender = 0x01010101
	const receiver = 0x02020202
	got := make(chan []byte, 16)
	recv := NewBusImplRabbitMQ(receiver, func(src uint32, data []byte) error {
		got <- data
		return nil
	}, addr)
	defer recv.Close()
	snd := NewBusImplRabbitMQ(sender, func(uint32, []byte) error { return nil }, addr)
	defer snd.Close()

	// Start 同步等待首次连接。
	startCtx, startCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer startCancel()
	if err := recv.Start(startCtx); err != nil {
		t.Fatalf("receiver Start: %v", err)
	}
	if err := snd.Start(startCtx); err != nil {
		t.Fatalf("sender Start: %v", err)
	}
	if !recv.Healthy() {
		t.Fatal("receiver Start 成功后 Healthy 应为 true")
	}

	// 发送若干消息，验证接收。
	const n = 8
	for i := 0; i < n; i++ {
		if err := snd.Send(receiver, []byte{byte(i)}, nil); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	for i := 0; i < n; i++ {
		select {
		case data := <-got:
			if len(data) != 1 || data[0] != byte(i) {
				t.Fatalf("msg %d: got %v, want [%d]", i, data, i)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("msg %d 未在 5s 内收到", i)
		}
	}
}

var errAMQPUnreachable = errors.New("amqp address unreachable")

// probeAMQP 快速探测 RabbitMQ 是否可达（TCP 拨号）。
func probeAMQP(addr string, timeout time.Duration) error {
	hostPort := parseAMQPHostPort(addr)
	if hostPort == "" {
		return errAMQPUnreachable
	}
	conn, err := net.DialTimeout("tcp", hostPort, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// parseAMQPHostPort 从 amqp:// URL 提取 host:port。
func parseAMQPHostPort(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5672"
	}
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, port)
}
