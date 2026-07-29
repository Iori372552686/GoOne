package ws_server

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/util/bufpool"
)

// noopWsHandler 满足 IWsTcpSvrEventHandler，用于生命周期测试。
type noopWsHandler struct {
	onConn func(net.Conn)
}

func (h *noopWsHandler) OnConn(conn net.Conn) {
	if h.onConn != nil {
		h.onConn(conn)
	}
}
func (h *noopWsHandler) OnRead(net.Conn, []byte) int { return 0 }
func (h *noopWsHandler) OnClose(net.Conn)            {}

func wsFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestWsRunGinWsReportsPortConflictAtStart 验证 P0-04：RunGinWs 同步 net.Listen，端口冲突
// 必须在 Start（而非稍后的 goroutine 内）返回。
func TestWsRunGinWsReportsPortConflictAtStart(t *testing.T) {
	port := wsFreePort(t)
	// 先占用端口。WS server 绑定所有接口（":port"），故 blocker 也必须绑所有接口才能
	// 真正冲突（0.0.0.0:port 与 127.0.0.1:port 在某些 OS 上可共存）。
	blocker, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("blocker listen: %v", err)
	}
	defer blocker.Close()

	svr := &WsTcpSvr{handler: &noopWsHandler{}}
	svr.accepting.Store(true)
	svr.mapOfConnInfo = make(map[net.Conn]*wsConnEntry)
	err = svr.RunGinWs("release", port)
	if err == nil {
		// 旧实现用 go router.Run 异步绑定，Start 永不返回端口冲突；这里给一点时间
		// 让后台 goroutine（若有）跑完，再判定。
		time.Sleep(100 * time.Millisecond)
		svr.Stop()
		t.Fatal("期望端口冲突在 Start 期同步返回 error，实际 nil（疑似仍用异步 ListenAndServe）")
	}
}

// TestWsQuiesceClosesListenerKeepsUpgradeRejecting 验证 P0-04：Quiesce 关闭 HTTP listener
// 后新 Upgrade 被拒绝（accepting=false），既有 WS 连接保留。
func TestWsQuiesceClosesListenerKeepsUpgradeRejecting(t *testing.T) {
	port := wsFreePort(t)
	svr := &WsTcpSvr{handler: &noopWsHandler{}}
	svr.accepting.Store(true)
	svr.mapOfConnInfo = make(map[net.Conn]*wsConnEntry)
	if err := svr.RunGinWs("release", port); err != nil {
		t.Fatalf("RunGinWs: %v", err)
	}
	// Quiesce 前 listener 已就绪。
	if svr.httpListener == nil {
		t.Fatal("httpListener 应已保存")
	}
	svr.Quiesce()
	if svr.accepting.Load() {
		t.Fatal("Quiesce 后 accepting 必须为 false")
	}
	if svr.httpListener != nil {
		t.Fatal("Quiesce 后 httpListener 应已关闭并置 nil")
	}
	// Quiesce 后 httpServer 仍保留（供 Stop 强制 Close）。
	if svr.httpServer == nil {
		t.Fatal("Quiesce 后 httpServer 应保留供 Stop 强关")
	}
	svr.Stop()
}

// TestWsStopForceClosesHttpServer 验证 P0-04：Stop 强制 Close httpServer 并清空指针。
func TestWsStopForceClosesHttpServer(t *testing.T) {
	port := wsFreePort(t)
	svr := &WsTcpSvr{handler: &noopWsHandler{}}
	svr.accepting.Store(true)
	svr.mapOfConnInfo = make(map[net.Conn]*wsConnEntry)
	if err := svr.RunGinWs("release", port); err != nil {
		t.Fatalf("RunGinWs: %v", err)
	}
	svr.Stop()
	if svr.httpServer != nil {
		t.Fatal("Stop 后 httpServer 应置 nil")
	}
	// Stop 后端口应可被再次绑定（server 已 Close）。
	relisten, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("Stop 后端口应可重新绑定，got %v", err)
	}
	_ = relisten.Close()
}

// TestWsDestroyConnDoesNotCloseChannel 验证 P0-04：destroyConn 不调用 close(chanWrite)，
// 而是向 chanWrite 投递 nil（关闭信号）。这从根上消除 send-on-closed-channel 竞态——
// 即使并发 WriteData 正在 send，channel 也永远不会被 close。
func TestWsDestroyConnDoesNotCloseChannel(t *testing.T) {
	svr := &WsTcpSvr{handler: &noopWsHandler{}}
	svr.mapOfConnInfo = make(map[net.Conn]*wsConnEntry)
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	entry := &wsConnEntry{chanWrite: make(chan *bufpool.Buffer, 1)}
	svr.mapOfConnInfo[c1] = entry

	svr.destroyConn(c1)

	// channel 不应被 close：再 send 不应 panic。
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("destroyConn 关闭了 channel，send panic: %v", r)
			}
		}()
		// entry 已从 map 删除，但 entry 指针仍持有；直接向其 chanWrite send 验证未 close。
		select {
		case entry.chanWrite <- bufpool.Acquire(1):
		default:
		}
	}()
	// 释放测试 lease。
	if len(entry.chanWrite) > 0 {
		bufpool.Release(<-entry.chanWrite)
	}
}

// TestWsDestroyConnSendsCloseSignal 验证：destroyConn 向 chanWrite 投递 nil，使写协程
// 能收到关闭信号退出（而非依赖 close(chan) 的 ok=false）。
func TestWsDestroyConnSendsCloseSignal(t *testing.T) {
	svr := &WsTcpSvr{handler: &noopWsHandler{}}
	svr.mapOfConnInfo = make(map[net.Conn]*wsConnEntry)
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	entry := &wsConnEntry{chanWrite: make(chan *bufpool.Buffer, 2)}
	svr.mapOfConnInfo[c1] = entry

	svr.destroyConn(c1)
	// chanWrite 应收到一个 nil（关闭信号）。
	select {
	case got := <-entry.chanWrite:
		if got != nil {
			t.Fatalf("期望 nil 关闭信号，got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("destroyConn 未向 chanWrite 投递 nil 关闭信号")
	}
}
