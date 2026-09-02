package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
)

// redisTestAddr/redisTestPass 取自环境变量（默认本地无密码实例），
// 便于在不同环境下运行容量集成测试。
func redisTestAddr() (host string, port int, addr string, pass string) {
	addr = os.Getenv("GOONE_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	pass = os.Getenv("GOONE_REDIS_PASS")
	host = addr
	port = 6379
	if i := lastColon(addr); i >= 0 {
		host = addr[:i]
		if p, err := strconv.Atoi(addr[i+1:]); err == nil {
			port = p
		}
	}
	return host, port, addr, pass
}

func lastColon(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

// capKeyPrefix 容量测试 key 的命名空间前缀：共享 Redis 中与业务 key 隔离，
// 便于识别与批量清理（历史版本无前缀裸写 0x%08x，污染过 dev 实例）。
const capKeyPrefix = "goone:test:cap:"

func capKey(i int) string { return fmt.Sprintf("%s0x%08x", capKeyPrefix, i) }

func TestCap(t *testing.T) {
	host, port, addr, pass := redisTestAddr()
	itest.Require(t, addr)

	const (
		c      = 1 * 1024
		n      = 100
		ttlSec = 600 // 兜底 TTL：测试进程被杀也不会留下永久 key
	)

	b := [c]byte{}
	for i := 0; i < c; i++ {
		b[i] = byte(i % 256)
	}

	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(context.Background(), Config{InstanceID: 1, IP: host, Port: port, Password: pass})
	if err != nil {
		t.Skipf("redis unavailable, skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < n; i++ {
			_ = redisMgr.Delete(context.Background(), 1, capKey(i))
		}
		_ = redisMgr.Close()
	})
	now := time.Now()
	for i := 0; i < n; i++ {
		err = redisMgr.SetBytes(context.Background(), 1, capKey(i), b[:], ttlSec*time.Second)
		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println(time.Since(now))
}

func TestIncBy(t *testing.T) {
	host, port, addr, pass := redisTestAddr()
	itest.Require(t, addr)

	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(context.Background(), Config{InstanceID: 1, IP: host, Port: port, Password: pass})
	if err != nil {
		t.Skipf("redis unavailable, skipping integration test: %v", err)
	}
	t.Cleanup(func() {
		_ = redisMgr.Delete(context.Background(), 1, "IncrTest2")
		_ = redisMgr.Close()
	})

	for i := 1; i <= 24; i++ {
		ret, err := redisMgr.IncrBy(context.Background(), 1, "IncrTest2", 2)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("ret =%v\n", ret)
	}
}
