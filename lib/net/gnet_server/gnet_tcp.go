// gnet_tcp.go implements the event-driven TCP transport backend
// (gnet v1, epoll/kqueue based): no per-connection goroutines, frames are
// split by a length-field codec inside the event loop.
//
// The gnet connection is wrapped into a net.Conn adapter so the session
// layer (net_mgr.ConnTcpSvr) works identically on both the goroutine-based
// (gonet) and the event-driven (gnet) backend.
package gnet_svr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/panjf2000/gnet"
)

// TcpEventHandler is the session-facing callback set (net.Conn view).
type TcpEventHandler interface {
	OnConn(net.Conn)
	OnPacket(net.Conn, []byte) // 在事件循环内同步调用；不得保留 data 引用
	OnClose(net.Conn)
}

// TcpServer is an event-driven TCP server with length-field framing.
type TcpServer struct {
	gnet.EventServer

	addr    string
	codec   *lengthFieldCodec
	handler TcpEventHandler

	ready chan struct{}

	// accepting 是 admission gate（P0-06）：Quiesce 置 false 后，OnOpened 立即关闭新
	// 连接，使网关排空期不再接收新客户端。Start 时为 true。
	accepting atomic.Bool
	// stopped 保证 Stop 幂等。
	stopped atomic.Bool
}

// NewTcpServer creates a gnet-backed TCP server. headerLen/bodyLen describe
// the frame layout (same contract as tcp_server.TcpPacketInfo).
func NewTcpServer(headerLen int, bodyLen func(header []byte) int, handler TcpEventHandler) *TcpServer {
	return &TcpServer{
		codec:   &lengthFieldCodec{headerLen: headerLen, bodyLen: bodyLen},
		handler: handler,
		ready:   make(chan struct{}),
	}
}

// Start launches the event loop and waits until it is accepting (or fails).
func (s *TcpServer) Start(ip string, port int) error {
	if s.handler == nil {
		return errors.New("gnet tcp server requires a handler")
	}
	s.addr = fmt.Sprintf("tcp://%s:%d", ip, port)
	s.accepting.Store(true)

	errCh := make(chan error, 1)
	go func() {
		err := gnet.Serve(s, s.addr,
			gnet.WithMulticore(true),
			gnet.WithCodec(s.codec),
			gnet.WithTCPNoDelay(gnet.TCPNoDelay),
		)
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-s.ready:
		logger.Infof("gnet tcp server listening on %s", s.addr)
		return nil
	case err := <-errCh:
		return fmt.Errorf("gnet serve failed: %w", err)
	case <-time.After(3 * time.Second):
		return errors.New("gnet tcp server start timeout")
	}
}

// Quiesce 关闭 admission gate：新连接在 OnOpened 被立即关闭，但既有连接保留（P0-06）。
// 幂等。
func (s *TcpServer) Quiesce() {
	s.accepting.Store(false)
}

// Stop shuts the event loop down. P0-06：幂等，关闭 admission gate 后停止 gnet。
func (s *TcpServer) Stop() error {
	s.accepting.Store(false)
	if !s.stopped.CompareAndSwap(false, true) {
		return nil
	}
	return gnet.Stop(context.Background(), s.addr)
}

// WriteData merges data1+data2 and writes asynchronously via the event loop.
// It satisfies the net_mgr transport contract.
//
// 注意：gnet v1 的 AsyncWrite 会持有缓冲直到事件循环写完，且没有完成回调，
// 因此这里不使用 bufpool（无法确定归还时机），由 GC 回收。
func (s *TcpServer) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	adapter, ok := conn.(*gnetConn)
	if !ok {
		return errors.New("conn is not a gnet connection")
	}
	merged := make([]byte, len(data1)+len(data2))
	pos := copy(merged, data1)
	copy(merged[pos:], data2)
	return adapter.c.AsyncWrite(merged)
}

// Close closes the underlying gnet connection (transport contract).
func (s *TcpServer) Close(conn net.Conn) error {
	adapter, ok := conn.(*gnetConn)
	if !ok {
		return errors.New("conn is not a gnet connection")
	}
	return adapter.c.Close()
}

// ---- gnet.EventHandler ----

func (s *TcpServer) OnInitComplete(_ gnet.Server) gnet.Action {
	close(s.ready)
	return gnet.None
}

func (s *TcpServer) OnOpened(c gnet.Conn) ([]byte, gnet.Action) {
	// P0-06 admission gate：Quiesce 后新连接立即关闭，不进入 handler。
	if !s.accepting.Load() {
		return nil, gnet.Close
	}
	adapter := &gnetConn{c: c}
	c.SetContext(adapter)
	s.handler.OnConn(adapter)
	return nil, gnet.None
}

func (s *TcpServer) OnClosed(c gnet.Conn, _ error) gnet.Action {
	if adapter, ok := c.Context().(*gnetConn); ok {
		s.handler.OnClose(adapter)
	}
	return gnet.None
}

func (s *TcpServer) React(frame []byte, c gnet.Conn) ([]byte, gnet.Action) {
	adapter, ok := c.Context().(*gnetConn)
	if !ok {
		return nil, gnet.Close
	}
	// frame 底层缓冲归事件循环所有，handler 同步消费、不得保留引用
	// （与 gonet 后端的 OnPacket 约定一致）。
	s.handler.OnPacket(adapter, frame)
	return nil, gnet.None
}

// ---- length-field codec ----

var errIncompleteFrame = errors.New("incomplete frame")

type lengthFieldCodec struct {
	headerLen int
	bodyLen   func(header []byte) int
}

func (cc *lengthFieldCodec) Encode(_ gnet.Conn, buf []byte) ([]byte, error) {
	return buf, nil
}

func (cc *lengthFieldCodec) Decode(c gnet.Conn) ([]byte, error) {
	size, header := c.ReadN(cc.headerLen)
	if size < cc.headerLen {
		return nil, errIncompleteFrame
	}

	bodyLen := cc.bodyLen(header)
	if bodyLen < 0 {
		logger.Warningf("Received an invalid gnet packet header, closing connection {remote:%v}", c.RemoteAddr())
		_ = c.Close()
		return nil, errors.New("invalid frame header")
	}

	total := cc.headerLen + bodyLen
	size, frame := c.ReadN(total)
	if size < total {
		return nil, errIncompleteFrame
	}
	c.ShiftN(total)
	return frame, nil
}

// ---- net.Conn adapter ----

// gnetConn adapts gnet.Conn to net.Conn for the session layer. Reads are not
// supported (data is pushed through OnPacket by the event loop); writes go
// through AsyncWrite and are therefore thread-safe.
type gnetConn struct {
	c gnet.Conn
}

var _ net.Conn = (*gnetConn)(nil)

func (g *gnetConn) Read(_ []byte) (int, error) {
	return 0, errors.New("gnet conn does not support synchronous read")
}

func (g *gnetConn) Write(b []byte) (int, error) {
	// AsyncWrite 持有 b 直到写完成：拷贝以保证调用方可复用其缓冲。
	buf := make([]byte, len(b))
	copy(buf, b)
	if err := g.c.AsyncWrite(buf); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (g *gnetConn) Close() error         { return g.c.Close() }
func (g *gnetConn) LocalAddr() net.Addr  { return g.c.LocalAddr() }
func (g *gnetConn) RemoteAddr() net.Addr { return g.c.RemoteAddr() }

func (g *gnetConn) SetDeadline(time.Time) error      { return nil }
func (g *gnetConn) SetReadDeadline(time.Time) error  { return nil }
func (g *gnetConn) SetWriteDeadline(time.Time) error { return nil }
