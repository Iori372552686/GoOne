package net_mgr

import (
	"errors"
	"net"
	"testing"
	"time"
)

// TestConstructorsNeverProduceNilHub 验证 三个旧构造器默认创建非 nil hub，
// 兼容调用方不再因 nil hub 触发空指针或跳过 admission。
func TestConstructorsNeverProduceNilHub(t *testing.T) {
	if tcp := NewTcpSvr(); tcp.hub == nil {
		t.Fatal("NewTcpSvr produced nil hub")
	}
	if ws := NewWsTcpSvr(); ws.hub == nil {
		t.Fatal("NewWsTcpSvr produced nil hub")
	}
	if kcp := NewKcpSvr(); kcp.hub == nil {
		t.Fatal("NewKcpSvr produced nil hub")
	}
	// lease 也必须初始化。
	if NewTcpSvr().lease == nil || NewWsTcpSvr().lease == nil || NewKcpSvr().lease == nil {
		t.Fatal("constructors must initialize connection lease")
	}
}

// TestWithHubConstructorsRejectNilHub 验证 显式 *WithHub 构造器拒绝 nil hub。
func TestWithHubConstructorsRejectNilHub(t *testing.T) {
	if _, err := NewTcpSvrWithHub(nil); !errors.Is(err, errNilHub) {
		t.Fatalf("NewTcpSvrWithHub(nil) err=%v, want errNilHub", err)
	}
	if _, err := NewWsTcpSvrWithHub(nil); !errors.Is(err, errNilHub) {
		t.Fatalf("NewWsTcpSvrWithHub(nil) err=%v, want errNilHub", err)
	}
	if _, err := NewKcpSvrWithHub(nil); !errors.Is(err, errNilHub) {
		t.Fatalf("NewKcpSvrWithHub(nil) err=%v, want errNilHub", err)
	}

	hub := NewSessionHub(noopCounter{})
	if _, err := NewTcpSvrWithHub(hub); err != nil {
		t.Fatalf("NewTcpSvrWithHub(non-nil) err=%v", err)
	}
	if _, err := NewWsTcpSvrWithHub(hub); err != nil {
		t.Fatalf("NewWsTcpSvrWithHub(non-nil) err=%v", err)
	}
	if _, err := NewKcpSvrWithHub(hub); err != nil {
		t.Fatalf("NewKcpSvrWithHub(non-nil) err=%v", err)
	}
}

// connTracker 是一个 ActivityCounter，记录 Inc/Dec 调用以校验 lease 行为。
type connTracker struct {
	conns int64
	sess  int64
}

func (c *connTracker) ActiveConnections() int64 { return c.conns }
func (c *connTracker) ActiveSessions() int64    { return c.sess }
func (c *connTracker) IncConnection()           { c.conns++ }
func (c *connTracker) DecConnection()           { c.conns-- }
func (c *connTracker) IncSession()              { c.sess++ }
func (c *connTracker) DecSession()              { c.sess-- }

// TestRejectedConnectionDoesNotDecrement 验证 被 admission 拒绝的连接不增加计数，
// OnClose 也不释放；只有 admitted 连接的 OnClose 才 DecConnection/Release。
//
// 这是 TCP/WS/KCP 三传输共享的 admission lease 契约。这里直接驱动 OnConn/OnClose 验证
// lease 逻辑，不依赖真实网络监听。
func TestRejectedConnectionDoesNotDecrement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*SessionHub) admittedTransport
	}{
		{"tcp", func(h *SessionHub) admittedTransport { return newTcpForLeaseTest(h) }},
		{"ws", func(h *SessionHub) admittedTransport { return newWsForLeaseTest(h) }},
		{"kcp", func(h *SessionHub) admittedTransport { return newKcpForLeaseTest(h) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := &connTracker{}
			hub := NewSessionHub(tr)
			// admission enforce，上限 1：第一个 admitted，第二个拒绝。
			hub.SetAdmission(NewAdmissionController(hub, AdmissionLimits{
				MaxConnections: 1, OverloadMode: OverloadModeEnforce,
			}))
			gw := tc.build(hub)

			conn1 := &fakeConn{}
			conn2 := &fakeConn{} // 会被拒绝

			// 第一个连接 admitted。
			gw.OnConn(conn1)
			if tr.conns != 1 {
				t.Fatalf("after admit conn1, conns=%d want 1", tr.conns)
			}
			// 第二个连接被拒绝，不得增加计数。
			gw.OnConn(conn2)
			if tr.conns != 1 {
				t.Fatalf("after reject conn2, conns=%d want 1 (reject must not increment)", tr.conns)
			}
			// OnClose 被拒绝的 conn2：不得释放计数。
			gw.OnClose(conn2)
			if tr.conns != 1 {
				t.Fatalf("OnClose rejected conn2 must not decrement, conns=%d want 1", tr.conns)
			}
			// OnClose admitted conn1：释放计数。
			gw.OnClose(conn1)
			if tr.conns != 0 {
				t.Fatalf("OnClose admitted conn1 must decrement, conns=%d want 0", tr.conns)
			}
		})
	}
}

// admittedTransport 是三传输在 OnConn/OnClose 上共享的最小接口，供 lease 契约测试复用。
type admittedTransport interface {
	OnConn(conn net.Conn)
	OnClose(conn net.Conn)
}

func newTcpForLeaseTest(hub *SessionHub) *ConnTcpSvr {
	s, err := NewTcpSvrWithHub(hub)
	if err != nil {
		panic(err)
	}
	return s
}
func newWsForLeaseTest(hub *SessionHub) *ConnWsTcpSvr {
	s, err := NewWsTcpSvrWithHub(hub)
	if err != nil {
		panic(err)
	}
	return s
}
func newKcpForLeaseTest(hub *SessionHub) *ConnKcpSvr {
	s, err := NewKcpSvrWithHub(hub)
	if err != nil {
		panic(err)
	}
	return s
}

// fakeConn 是一个最小 net.Conn 实现，仅满足 lease 测试（RemoteAddr/Close 等）。
type fakeConn struct {
	closed bool
}

func (f *fakeConn) Read([]byte) (int, error)        { return 0, nil }
func (f *fakeConn) Write([]byte) (int, error)       { return 0, nil }
func (f *fakeConn) Close() error                    { f.closed = true; return nil }
func (f *fakeConn) LocalAddr() net.Addr             { return dummyAddr{} }
func (f *fakeConn) RemoteAddr() net.Addr            { return dummyAddr{} }
func (f *fakeConn) SetDeadline(time.Time) error     { return nil }
func (f *fakeConn) SetReadDeadline(time.Time) error { return nil }
func (f *fakeConn) SetWriteDeadline(time.Time) error {
	return nil
}

type dummyAddr struct{}

func (dummyAddr) Network() string { return "fake" }
func (dummyAddr) String() string  { return "fake" }
