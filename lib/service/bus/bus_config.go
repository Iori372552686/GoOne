package bus

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RabbitMQConfig for RabbitMQ bus backend.
type RabbitMQConfig struct {
	Addr string `json:"addr" yaml:"addr"`
}

// NSQConfig for NSQ bus backend.
type NSQConfig struct {
	LookupAddrs []string `json:"lookup_addrs" yaml:"lookup_addrs"`
	NsqdAddr    string   `json:"nsqd_addr" yaml:"nsqd_addr"` // host:port
	TopicPrefix string   `json:"topic_prefix" yaml:"topic_prefix"`
	Channel     string   `json:"channel" yaml:"channel"`
	Concurrency int      `json:"concurrency" yaml:"concurrency"`
	// MaxInFlight 最大在途消息数（<=0 时用默认值 2）。
	MaxInFlight int `json:"max_in_flight" yaml:"max_in_flight"`
	// LookupdPollInterval nsqlookupd 轮询间隔，格式如 "3s"（空时用默认 3s）。
	LookupdPollInterval string `json:"lookupd_poll_interval" yaml:"lookupd_poll_interval"`
}

// NatsConfig for NATS bus backend.
type NatsConfig struct {
	URL           string `json:"url" yaml:"url"` // nats://host:port
	SubjectPrefix string `json:"subject_prefix" yaml:"subject_prefix"`
	QueueGroup    string `json:"queue_group" yaml:"queue_group"`
}

// KafkaConfig for Kafka bus backend.
type KafkaConfig struct {
	Brokers       []string `json:"brokers" yaml:"brokers"`
	TopicPrefix   string   `json:"topic_prefix" yaml:"topic_prefix"`
	GroupIDPrefix string   `json:"group_id_prefix" yaml:"group_id_prefix"`
}

// RocketMQConfig for RocketMQ bus backend.
type RocketMQConfig struct {
	NameServers   []string `json:"name_servers" yaml:"name_servers"`
	Topic         string   `json:"topic" yaml:"topic"`
	ConsumerGroup string   `json:"consumer_group" yaml:"consumer_group"`
}

// ParseAddr parses a single bus addr string into (implType, backendConfig).
//
// Supported examples:
// - amqp://guest:guest@127.0.0.1:5672/                         (rabbitmq)
// - rabbitmq://?addr=amqp://guest:guest@127.0.0.1:5672/        (rabbitmq)
// - nats://127.0.0.1:4222?subject_prefix=bus&queue_group=g1    (nats)
// - kafka://127.0.0.1:9092,127.0.0.2:9092?topic_prefix=bus     (kafka)
// - rocketmq://127.0.0.1:9876?topic=goone_bus&consumer_group=goone_bus  (rocketmq)
// - nsq://127.0.0.1:4150?lookup=127.0.0.1:4161&topics=test&chan=ch&concurrency=3 (nsq)
func ParseAddr(addr string) (implType string, cfg any, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", nil, fmt.Errorf("bus addr is empty")
	}

	// Treat raw amqp URL as rabbitmq directly.
	if strings.HasPrefix(addr, "amqp://") || strings.HasPrefix(addr, "amqps://") {
		return "rabbitmq", RabbitMQConfig{Addr: addr}, nil
	}

	u, err := url.Parse(addr)
	if err != nil {
		return "", nil, err
	}

	switch strings.ToLower(u.Scheme) {
	case "rabbitmq":
		q := u.Query()
		amqpAddr := strings.TrimSpace(q.Get("addr"))
		if amqpAddr == "" {
			return "", nil, fmt.Errorf("rabbitmq missing addr (use rabbitmq://?addr=amqp://...)")
		}
		return "rabbitmq", RabbitMQConfig{Addr: amqpAddr}, nil

	case "nats":
		q := u.Query()
		return "nats", NatsConfig{
			URL:           "nats://" + u.Host + u.Path,
			SubjectPrefix: strings.TrimSpace(q.Get("subject_prefix")),
			QueueGroup:    strings.TrimSpace(q.Get("queue_group")),
		}, nil

	case "kafka":
		hostPart := strings.TrimPrefix(addr, "kafka://")
		hostPart = strings.Split(hostPart, "?")[0]
		q := u.Query()
		return "kafka", KafkaConfig{
			Brokers:       splitCSV(hostPart),
			TopicPrefix:   strings.TrimSpace(q.Get("topic_prefix")),
			GroupIDPrefix: strings.TrimSpace(q.Get("group_id_prefix")),
		}, nil

	case "rocketmq":
		hostPart := strings.TrimPrefix(addr, "rocketmq://")
		hostPart = strings.Split(hostPart, "?")[0]
		q := u.Query()
		return "rocketmq", RocketMQConfig{
			NameServers:   splitCSV(hostPart),
			Topic:         strings.TrimSpace(q.Get("topic")),
			ConsumerGroup: strings.TrimSpace(q.Get("consumer_group")),
		}, nil

	case "nsq":
		// nsqd address
		host := u.Host
		if host == "" {
			return "", nil, fmt.Errorf("nsq missing nsqd host:port")
		}
		q := u.Query()
		concurrency := 0
		if v := strings.TrimSpace(q.Get("concurrency")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				concurrency = n
			}
		}
		maxInFlight := 0
		if v := strings.TrimSpace(q.Get("max_in_flight")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxInFlight = n
			}
		}
		return "nsq", NSQConfig{
			NsqdAddr:            host,
			LookupAddrs:         splitCSV(q.Get("lookup")),
			TopicPrefix:         strings.TrimSpace(q.Get("topics")),
			Channel:             strings.TrimSpace(q.Get("chan")),
			Concurrency:         concurrency,
			MaxInFlight:         maxInFlight,
			LookupdPollInterval: strings.TrimSpace(q.Get("lookupd_poll_interval")),
		}, nil

	default:
		return "", nil, fmt.Errorf("unsupported bus scheme: %q", u.Scheme)
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(strings.TrimSpace(s), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
