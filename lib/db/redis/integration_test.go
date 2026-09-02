package redis

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisV9DataCompatibilityIntegration(t *testing.T) {
	host, port, addr, pass := redisTestAddr()
	itest.Require(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m := NewRedisMgr()
	if err := m.AddInstance(ctx, Config{InstanceID: 1, IP: host, Port: port, Password: pass}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })

	prefix := fmt.Sprintf("goone:test:redis-v9:%d", time.Now().UnixNano())
	keys := []string{prefix + ":string", prefix + ":missing", prefix + ":hash", prefix + ":counter", prefix + ":zset", prefix + ":legacy"}
	t.Cleanup(func() { _ = m.Delete(context.Background(), 1, keys...) })

	binaryValue := []byte{0, 1, 2, 255, 10}
	if err := m.SetBytes(ctx, 1, keys[0], binaryValue, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetBytes(ctx, 1, keys[0])
	if err != nil || !bytes.Equal(got, binaryValue) {
		t.Fatalf("GetBytes() = %v, %v", got, err)
	}
	values, err := m.MGetBytes(ctx, 1, keys[0], keys[1])
	if err != nil || len(values) != 2 || !bytes.Equal(values[0], binaryValue) || values[1] != nil {
		t.Fatalf("MGetBytes() = %#v, %v", values, err)
	}

	if err := m.HSetBytes(ctx, 1, keys[2], "proto", binaryValue); err != nil {
		t.Fatal(err)
	}
	fields, err := m.HGetAllBytes(ctx, 1, keys[2])
	if err != nil || !bytes.Equal(fields["proto"], binaryValue) {
		t.Fatalf("HGetAllBytes() = %#v, %v", fields, err)
	}
	if value, err := m.IncrBy(ctx, 1, keys[3], 3); err != nil || value != 3 {
		t.Fatalf("IncrBy() = %d, %v", value, err)
	}
	if err := m.ZAdd(ctx, 1, keys[4], goredis.Z{Score: 2, Member: "b"}, goredis.Z{Score: 1, Member: "a"}); err != nil {
		t.Fatal(err)
	}
	if values, err := m.ZRange(ctx, 1, keys[4], 0, -1); err != nil || len(values) != 2 || values[0] != "a" {
		t.Fatalf("ZRange() = %#v, %v", values, err)
	}

	client, err := m.Client(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, keys[5], binaryValue, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if got, err := m.GetBytes(ctx, 1, keys[5]); err != nil || !bytes.Equal(got, binaryValue) {
		t.Fatalf("existing value compatibility = %v, %v", got, err)
	}
	if ttl, err := client.TTL(ctx, keys[0]).Result(); err != nil || ttl <= 0 {
		t.Fatalf("TTL() = %v, %v", ttl, err)
	}
}
