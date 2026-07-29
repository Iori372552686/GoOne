package net_mgr

import (
	"net"
	"testing"
)

// benchHub 构建一个填充了 N 个会话的 SessionHub，返回 hub 与底层连接列表（用于
// benchmark 的 lookup/bind/broadcast）。
func benchHub(b *testing.B, n int) (*SessionHub, []net.Conn) {
	b.Helper()
	hub := NewSessionHub(noopCounter{})
	conns := make([]net.Conn, 0, n)
	for i := 0; i < n; i++ {
		c := newPipeB(b, fakeAddrStr(50000+i))
		conns = append(conns, c)
		if _, _, err := hub.BindClient(c, uint64(1000+i), 1); err != nil {
			b.Fatalf("BindClient: %v", err)
		}
	}
	return hub, conns
}

// newPipeB 是 newPipe 的 testing.B 变体（newPipe 接收 *testing.T）。
func newPipeB(tb testing.TB, addr string) net.Conn {
	tb.Helper()
	c1, c2 := net.Pipe()
	return &fakeAddrConn{Conn: c1, remote: fakeAddr(addr), local: fakeAddr("127.0.0.1:0"), peer: c2}
}

func fakeAddrStr(port int) string {
	return "127.0.0.1:" + itoaPort(port)
}

func itoaPort(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BenchmarkSessionHubLookup 测量 GetClientByUid 的锁内查找成本。目标 0 alloc。
func BenchmarkSessionHubLookup(b *testing.B) {
	hub, _ := benchHub(b, 1000)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hub.GetClientByUid(uint64(1000 + (i % 1000)))
	}
}

// BenchmarkSessionHubBindReplace 测量同 UID 重绑（BindClient 返回 oldCli）的成本。
func BenchmarkSessionHubBindReplace(b *testing.B) {
	hub, conns := benchHub(b, 100)
	defer func() {
		for _, c := range conns {
			closePipe(c)
		}
	}()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := newPipeB(b, fakeAddrStr(60000+i))
		_, _, _ = hub.BindClient(c, uint64(1000+(i%100)), 1)
	}
}
