package bus_test

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/bus"
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/all"
)

func onRecvMsg(srcBusID uint32, data []byte) error {
	log.Printf("srcBusID:%v, data:%v", srcBusID, data)

	return nil
}

// requireBusITest skips unless BUS_ITEST=1 (integration tests need local MQ).
func requireBusITest(t *testing.T) {
	t.Helper()
	if os.Getenv("BUS_ITEST") != "1" {
		t.Skip("set BUS_ITEST=1 to run bus integration tests")
	}
}

func runBusSmoke(t *testing.T, addr string) {
	t.Helper()
	impl, err := bus.CreateBus(bus.IpStringToInt("1.1.2.2"), onRecvMsg, addr)
	if err != nil || impl == nil {
		t.Skipf("bus not available for %q: %v", addr, err)
	}

	if err := impl.Send(impl.SelfBusId(), []byte("abc"), nil); err != nil {
		t.Logf("Send error: %v", err)
	}

	time.Sleep(2 * time.Second)
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
