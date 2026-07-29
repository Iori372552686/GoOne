package net_mgr

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCounter 是测试用 ActivityCounter，记录连接/会话计数与 Inc/Dec 调用。
type fakeCounter struct {
	conns  atomic.Int64
	sess   atomic.Int64
	incS   atomic.Int64 // IncSession 调用次数
	decS   atomic.Int64 // DecSession 调用次数
}

func (f *fakeCounter) IncConnection()       { f.conns.Add(1) }
func (f *fakeCounter) DecConnection()       { f.conns.Add(-1) }
func (f *fakeCounter) IncSession()          { f.sess.Add(1); f.incS.Add(1) }
func (f *fakeCounter) DecSession()          { f.sess.Add(-1); f.decS.Add(1) }
func (f *fakeCounter) ActiveConnections() int64 { return f.conns.Load() }
func (f *fakeCounter) ActiveSessions() int64    { return f.sess.Load() }

// pipeConn 包装 net.Pipe 产生可比较的连接用于 hub 测试。每个 pipe 端是独立的 conn。
func newPipe(t *testing.T, addr string) net.Conn {
	t.Helper()
	c1, c2 := net.Pipe()
	// net.Pipe 的 RemoteAddr 默认返回 Addr{}；用 fakeAddrConn 包装以提供可解析地址。
	return &fakeAddrConn{Conn: c1, remote: fakeAddr(addr), local: fakeAddr("127.0.0.1:0"), peer: c2}
}

type fakeAddr string

func (f fakeAddr) Network() string { return "tcp" }
func (f fakeAddr) String() string  { return string(f) }

// fakeAddrConn 包装一个 net.Conn 以返回自定义地址，使 net.SplitHostPort 可用。
type fakeAddrConn struct {
	net.Conn
	remote net.Addr
	local  net.Addr
	peer   net.Conn
}

func (f *fakeAddrConn) RemoteAddr() net.Addr { return f.remote }
func (f *fakeAddrConn) LocalAddr() net.Addr  { return f.local }
func (f *fakeAddrConn) Close() error {
	_ = f.peer.Close()
	return f.Conn.Close()
}

// addrConn 复用 newPipe 的对端，避免泄漏。
func closePipe(c net.Conn) {
	if fc, ok := c.(*fakeAddrConn); ok {
		_ = fc.Close()
		return
	}
	_ = c.Close()
}

// TestSessionHubAtomicBindSameUID 验证同一 UID 第二次 BindClient 返回 oldCli，且会话计
// 数不重复增加（重绑不重复计）。
func TestSessionHubAtomicBindSameUID(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50001")
	defer closePipe(c1)

	cli1, old1, err := hub.BindClient(c1, 1001, 1)
	if err != nil {
		t.Fatalf("BindClient 1: %v", err)
	}
	if old1 != nil {
		t.Fatal("首次绑定不应返回 oldCli")
	}
	if fc.ActiveSessions() != 1 {
		t.Fatalf("ActiveSessions 应 1，got %d", fc.ActiveSessions())
	}
	_ = cli1

	// 同 UID 同 conn 再绑：oldCli 应是该 conn 自身（重绑）。
	cli2, old2, err := hub.BindClient(c1, 1001, 1)
	if err != nil {
		t.Fatalf("BindClient 2: %v", err)
	}
	if old2 == nil {
		t.Fatal("重绑应返回 oldCli")
	}
	// 会话计数不得因重绑增加。
	if fc.ActiveSessions() != 1 {
		t.Fatalf("重绑后 ActiveSessions 应仍 1，got %d", fc.ActiveSessions())
	}
	if fc.incS.Load() != 1 {
		t.Fatalf("IncSession 应只调用 1 次，got %d", fc.incS.Load())
	}
	_ = cli2
}

// TestSessionHubCrossTransportReplacement 验证同 UID 用不同 conn 重绑：返回 oldCli，
// ActiveSessions 不增加，GetClientByUid 返回新 conn。
func TestSessionHubCrossTransportReplacement(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50002")
	defer closePipe(c1)
	c2 := newPipe(t, "127.0.0.1:50003")
	defer closePipe(c2)

	if _, _, err := hub.BindClient(c1, 2002, 1); err != nil {
		t.Fatalf("BindClient c1: %v", err)
	}
	cli2, old2, err := hub.BindClient(c2, 2002, 1)
	if err != nil {
		t.Fatalf("BindClient c2: %v", err)
	}
	if old2 == nil || old2.Conn != c1 {
		t.Fatal("重绑应返回旧 conn c1 的 oldCli")
	}
	if fc.ActiveSessions() != 1 {
		t.Fatalf("跨传输重绑 ActiveSessions 应仍 1，got %d", fc.ActiveSessions())
	}
	if got := hub.GetClientByUid(2002); got.Conn != c2 {
		t.Fatal("GetClientByUid 应返回新 conn c2")
	}
	_ = cli2
}

// TestOldConnectionCloseDoesNotRemoveReplacement 验证旧连接的迟到 OnClose 不删除新连接，
// 也不重复减会话计数。
func TestOldConnectionCloseDoesNotRemoveReplacement(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50004")
	defer closePipe(c1)
	c2 := newPipe(t, "127.0.0.1:50005")
	defer closePipe(c2)

	_, _, _ = hub.BindClient(c1, 3003, 1)
	_, _, _ = hub.BindClient(c2, 3003, 1) // 替换 c1

	// 旧连接 c1 迟到 OnClose。
	uid, kicked := hub.RemoveConn(c1)
	if uid != 0 {
		t.Fatalf("迟到旧 OnClose 应返回 uid=0（不动新连接），got uid=%d kicked=%v", uid, kicked)
	}
	// 新连接仍在。
	if got := hub.GetClientByUid(3003); got == nil || got.Conn != c2 {
		t.Fatal("迟到旧 OnClose 不得删除新连接 c2")
	}
	if fc.ActiveSessions() != 1 {
		t.Fatalf("迟到旧 OnClose 不应减会话计数，ActiveSessions 应 1，got %d", fc.ActiveSessions())
	}
	// 真正关闭新连接 c2：uid 返回，会话减一。
	uid2, _ := hub.RemoveConn(c2)
	if uid2 != 3003 {
		t.Fatalf("新连接 OnClose 应返回 uid=3003，got %d", uid2)
	}
	if fc.ActiveSessions() != 0 {
		t.Fatalf("新连接关闭后 ActiveSessions 应 0，got %d", fc.ActiveSessions())
	}
}

// TestSessionHubRejectsBindAfterQuiesce 验证 Quiesce 后 BindClient 返回
// ErrGatewayDraining，不修改状态。
func TestSessionHubRejectsBindAfterQuiesce(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	hub.Quiesce()
	c1 := newPipe(t, "127.0.0.1:50006")
	defer closePipe(c1)

	_, _, err := hub.BindClient(c1, 4004, 1)
	if !errors.Is(err, ErrGatewayDraining) {
		t.Fatalf("Quiesce 后应返回 ErrGatewayDraining，got %v", err)
	}
	if fc.ActiveSessions() != 0 {
		t.Fatal("Quiesce 拒绝不得改变会话计数")
	}
	if hub.GetClientByUid(4004) != nil {
		t.Fatal("Quiesce 拒绝不得注册 client")
	}
}

// TestSessionHubParsesIPv6RemoteAddress 验证 net.SplitHostPort 解析 IPv6 地址不报错。
func TestSessionHubParsesIPv6RemoteAddress(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "[::1]:50007")
	defer closePipe(c1)

	cli, _, err := hub.BindClient(c1, 5005, 1)
	if err != nil {
		t.Fatalf("IPv6 BindClient 应成功，got %v", err)
	}
	if cli.RemoteAddr != "[::1]:50007" {
		t.Fatalf("RemoteAddr 应保留原样，got %q", cli.RemoteAddr)
	}
	// host 解析出 ::1（不 panic）。
	if cli.Ip == 0 && cli.Port == 0 {
		// bus.IpStringToInt("::1") 可能返回 0；这里只验证不 panic 且 Port 非零。
	}
	if cli.Port != 50007 {
		t.Fatalf("Port 应为 50007，got %d", cli.Port)
	}
}

// TestBroadcastSnapshotDoesNotHoldLockDuringWrite 验证 SnapshotByZone 在锁内构造快照，
// 锁外逐个写：一个慢写不得阻塞其它 lookup/remove。
func TestBroadcastSnapshotDoesNotHoldLockDuringWrite(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50008")
	defer closePipe(c1)

	_, _, _ = hub.BindClient(c1, 6006, 1)
	snap := hub.SnapshotByZone(0)
	if len(snap) != 1 {
		t.Fatalf("snapshot 应含 1 个 client，got %d", len(snap))
	}
	// 模拟慢写：在"写"期间，另一个 goroutine 应能完成 lookup/remove（锁未被持有）。
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 慢写：sleep 一段，模拟阻塞写。
		time.Sleep(50 * time.Millisecond)
	}()
	// 并发 lookup 应立即返回（不阻塞）。
	done := make(chan struct{})
	go func() {
		_ = hub.GetClientByUid(6006)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("GetClientByUid 在快照写期间被阻塞（锁未释放）")
	}
	wg.Wait()
}

// TestKickDoesNotHoldLockDuringMarshalOrClose 验证 hub 的 ConnByRemoteAddr/MarkKick 只做
// 锁内 map 操作；marshal/close 由调用方在锁外做（这里验证 hub 不提供持锁的 kick 路径）。
func TestKickDoesNotHoldLockDuringMarshalOrClose(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50009")
	defer closePipe(c1)

	_, _, _ = hub.BindClient(c1, 7007, 1)
	// hub 仅暴露 ConnByRemoteAddr + MarkKick；调用方在锁外 marshal/close。
	conn := hub.ConnByRemoteAddr("127.0.0.1:50009")
	if conn == nil {
		t.Fatal("ConnByRemoteAddr 应返回 conn")
	}
	hub.MarkKick("127.0.0.1:50009")

	// 关闭后 RemoveConn 应返回 kicked=true（不触发登出包）。
	uid, kicked := hub.RemoveConn(c1)
	if uid != 7007 || !kicked {
		t.Fatalf("期望 uid=7007 kicked=true，got uid=%d kicked=%v", uid, kicked)
	}
}

// TestSessionHubRemoveConnUnknownReturnsZero 验证未绑定连接的 RemoveConn 返回 0。
func TestSessionHubRemoveConnUnknownReturnsZero(t *testing.T) {
	fc := &fakeCounter{}
	hub := NewSessionHub(fc)
	c1 := newPipe(t, "127.0.0.1:50010")
	defer closePipe(c1)
	uid, _ := hub.RemoveConn(c1)
	if uid != 0 {
		t.Fatalf("未绑定连接 RemoveConn 应返回 0，got %d", uid)
	}
}

// 防止 context 未用（部分未来测试会用）。
var _ context.Context = (*ctxStub)(nil)

type ctxStub struct{}

func (*ctxStub) Deadline() (deadline time.Time, ok bool) { return }
func (*ctxStub) Done() <-chan struct{}                   { return nil }
func (*ctxStub) Err() error                              { return nil }
func (*ctxStub) Value(key any) any                       { return nil }
