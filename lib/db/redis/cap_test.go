package redis

import (
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

func TestCap(t *testing.T) {
	host, port, addr, pass := redisTestAddr()
	itest.Require(t, addr)

	const c = 1 * 1024

	b := [c]byte{}
	for i := 0; i < c; i++ {
		b[i] = byte(i % 256)
	}

	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(1, host, port, pass, 0, false)
	if err != nil {
		t.Skipf("redis unavailable, skipping integration test: %v", err)
	}
	now := time.Now()
	for i := 0; i < 100; i++ {
		err = redisMgr.SetBytes(1, fmt.Sprintf("0x%08x", i), b[:])
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
	err := redisMgr.AddInstance(1, host, port, pass, 0, false)
	if err != nil {
		t.Skipf("redis unavailable, skipping integration test: %v", err)
	}

	for i := 1; i <= 24; i++ {
		ret, err := redisMgr.IncrByKey(1, "IncrTest2", 2)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("ret =%v\n", ret)
	}
}
