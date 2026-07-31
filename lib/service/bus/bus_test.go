package bus_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/kafka"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/nats"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/nsq"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rocketmq"
)

func onRecvMsg(srcBusID uint32, data []byte) error {
	log.Printf("srcBusID:%v, data:%v", srcBusID, data)

	return nil
}

// requireBusITest skips unless GOONE_INTEGRATION=1 (integration tests need local MQ).
func requireBusITest(t *testing.T) {
	t.Helper()
	if !itest.Enabled() {
		t.Skip("set GOONE_INTEGRATION=1 to run bus integration tests")
	}
}

// fullRegistry 显式注册全部内置 driver，取代已删除的 driver/all blank import。
func fullRegistry() *bus.DriverRegistry {
	r := bus.NewDriverRegistry()
	r.MustRegister(rabbitmq.Driver())
	r.MustRegister(nsq.Driver())
	r.MustRegister(nats.Driver())
	r.MustRegister(kafka.Driver())
	r.MustRegister(rocketmq.Driver())
	return r
}

func runBusSmoke(t *testing.T, addr string) {
	t.Helper()
	impl, err := fullRegistry().CreateBus(bus.IpStringToInt("1.1.2.2"), onRecvMsg, addr)
	if err != nil || impl == nil {
		t.Skipf("bus not available for %q: %v", addr, err)
	}

	// Start 同步等待首次连接；失败即跳过（中间件不可达）。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := impl.Start(ctx); err != nil {
		t.Skipf("bus Start failed for %q: %v", addr, err)
	}

	if err := impl.Send(impl.SelfBusId(), []byte("abc"), nil); err != nil {
		t.Logf("Send error: %v", err)
	}

	_ = impl.Close()
}

func TestRabbitMQBus(t *testing.T) {
	requireBusITest(t)
	runBusSmoke(t, "amqp://guest:guest@127.0.0.1:5672/")
}

func TestNSQBus(t *testing.T) {
	requireBusITest(t)
	runBusSmoke(t, "nsq://127.0.0.1:4150?lookup=127.0.0.1:4161&topics=test&chan=ch&concurrency=1")
}

func TestNatsBus(t *testing.T) {
	requireBusITest(t)
	runBusSmoke(t, "nats://127.0.0.1:4222?subject_prefix=testbus&queue_group=test-group")
}

func TestKafkaBus(t *testing.T) {
	requireBusITest(t)
	runBusSmoke(t, "kafka://127.0.0.1:9092?topic_prefix=testbus&group_id_prefix=testgroup")
}

func TestRocketMQBus(t *testing.T) {
	requireBusITest(t)
	runBusSmoke(t, "rocketmq://127.0.0.1:9876?topic=testbus&consumer_group=testbus_group")
}

func TestParseAddr(t *testing.T) {
	cases := []struct {
		addr     string
		wantType string
	}{
		{"amqp://guest:guest@127.0.0.1:5672/", "rabbitmq"},
		{"rabbitmq://?addr=amqp://guest:guest@127.0.0.1:5672/", "rabbitmq"},
		{"nats://127.0.0.1:4222?subject_prefix=bus", "nats"},
		{"kafka://127.0.0.1:9092,127.0.0.2:9092?topic_prefix=bus", "kafka"},
		{"rocketmq://127.0.0.1:9876?topic=goone_bus", "rocketmq"},
		{"nsq://127.0.0.1:4150?lookup=127.0.0.1:4161", "nsq"},
	}
	for _, c := range cases {
		implType, cfg, err := bus.ParseAddr(c.addr)
		if err != nil {
			t.Fatalf("ParseAddr(%q) error: %v", c.addr, err)
		}
		if implType != c.wantType {
			t.Fatalf("ParseAddr(%q) type = %q, want %q", c.addr, implType, c.wantType)
		}
		if cfg == nil {
			t.Fatalf("ParseAddr(%q) returned nil config", c.addr)
		}
	}

	if _, _, err := bus.ParseAddr(""); err == nil {
		t.Fatal("expected error for empty addr")
	}
	if _, _, err := bus.ParseAddr("unknown://x"); err == nil {
		t.Fatal("expected error for unknown scheme")
	}
}
