package tcp_server

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

// noopHandler 满足 ITcpSvrEventHandler，用于 Quiesce/Stop 测试。
type noopHandler struct {
	onConn func(net.Conn)
}

func (h *noopHandler) OnConn(conn net.Conn) {
	if h.onConn != nil {
		h.onConn(conn)
	}
}
func (h *noopHandler) OnClose(conn net.Conn)                      {}
func (h *noopHandler) OnRead(conn net.Conn, data []byte) int      { return 0 }
func (h *noopHandler) OnRead2(conn net.Conn, data []byte) int     { return 0 }

// freePort 取一个本机空闲端口用于监听。
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestTcpSvrQuiesceRejectsNewConnections(t *testing.T) {
	port := freePort(t)
	svr := &TcpSvr{}
	connCount := 0
	var mu sync.Mutex
	handler := &noopHandler{onConn: func(net.Conn) {
		mu.Lock()
		connCount++
		mu.Unlock()
	}}
	if err := svr.InitAndRun("127.0.0.1", port, handler); err != nil {
		t.Fatalf("InitAndRun: %v", err)
	}

	// Quiesce 前可连接。
	c1, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("dial before quiesce: %v", err)
	}
	defer c1.Close()
	// 等待 OnConn 被回调。
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		if connCount > 0 {
			mu.Unlock()
			break
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
	}

	// Quiesce：停止接新连接。
	svr.Quiesce()
	// 再连应失败（listener 已关闭）。
	_, err = net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err == nil {
		t.Fatal("expected dial to fail after Quiesce (listener closed)")
	}

	// 幂等：再次 Quiesce 不 panic。
	svr.Quiesce()
	// Stop 关闭既有连接并清理。
	svr.Stop()
}

func TestTcpSvrStopClosesExistingConnections(t *testing.T) {
	port := freePort(t)
	svr := &TcpSvr{}
	handler := &noopHandler{}
	if err := svr.InitAndRun("127.0.0.1", port, handler); err != nil {
		t.Fatalf("InitAndRun: %v", err)
	}

	c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// 给一点时间让 server 侧注册连接。
	time.Sleep(50 * time.Millisecond)

	// Stop 应关闭既有连接。
	svr.Stop()
	// 客户端读应返回 EOF 或 error（连接被关）。
	buf := make([]byte, 1)
	if _, err := c.Read(buf); err == nil {
		t.Fatal("expected read error after Stop closed the connection")
	}
	_ = c.Close()

	// 幂等。
	svr.Stop()
}
