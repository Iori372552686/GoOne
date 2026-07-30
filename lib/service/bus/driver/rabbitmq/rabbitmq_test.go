package rabbitmq

import (
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

// TestDriverNameAndCtor 验证 P1-05：Driver() 返回的描述符名与 ctor 行为符合契约。
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
// ErrBusClosed。不连接真实 RabbitMQ（run 会重试但 StopCh 关闭后退出）。
func TestNewBusImplRabbitMQCloseIsIdempotent(t *testing.T) {
	// 用一个不可达地址，避免真实连接；run goroutine 会重试但 stopCh 关闭即退出。
	b := NewBusImplRabbitMQ(0x01010101, func(uint32, []byte) error { return nil }, "amqp://guest:guest@127.0.0.1:1/")
	// 给 run 一点时间进入重试循环。
	time.Sleep(100 * time.Millisecond)

	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// 幂等。
	if err := b.Close(); err != nil {
		t.Fatalf("second Close should be idempotent: %v", err)
	}
	// Close 后 Send 返回 ErrBusClosed。
	if err := b.Send(0x02020202, []byte("x"), nil); err != bus.ErrBusClosed {
		t.Fatalf("Close 后 Send 应返回 ErrBusClosed，got %v", err)
	}
	if b.Healthy() {
		t.Fatal("Close 后 Healthy 应为 false")
	}
}

// amqpTestAddr 取自环境变量（默认本地实例），便于在不同环境下运行真实联调。
func amqpTestAddr() string {
	if v := os.Getenv("GOONE_AMQP_ADDR"); v != "" {
		return v
	}
	return "amqp://guest:guest@127.0.0.1:5672/"
}

// TestRabbitMQRealIntegration 验证 P1-05：真实 RabbitMQ 联调——两个不同 bus ID 的实例
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

	// 等待连接建立。
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && !recv.Healthy() {
		time.Sleep(100 * time.Millisecond)
	}
	if !recv.Healthy() {
		t.Fatal("receiver 未在 8s 内连接 RabbitMQ")
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
