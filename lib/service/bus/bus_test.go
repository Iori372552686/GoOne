package bus_test

import (
	"testing"

	"github.com/Iori372552686/GoOne/lib/service/bus"
)

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
