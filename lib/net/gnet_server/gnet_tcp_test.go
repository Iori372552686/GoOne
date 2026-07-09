package gnet_svr

import (
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
)

type echoHandler struct {
	svr     *TcpServer
	opened  atomic.Int64
	closed  atomic.Int64
	packets atomic.Int64
}

func (h *echoHandler) OnConn(conn net.Conn) { h.opened.Add(1) }

func (h *echoHandler) OnPacket(conn net.Conn, data []byte) {
	h.packets.Add(1)
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	_ = h.svr.WriteData(conn, data[:headerLen], data[headerLen:])
}

func (h *echoHandler) OnClose(conn net.Conn) { h.closed.Add(1) }

func buildCSFrame(cmd uint32, body []byte) []byte {
	header := sharedstruct.CSPacketHeader{
		Uid:     10001,
		Cmd:     cmd,
		BodyLen: uint32(len(body)),
	}
	return append(header.ToBytes(), body...)
}

// TestGnetTcpEchoWithSticky verifies the event-driven backend end to end:
// codec frame split on sticky writes, adapter identity across callbacks and
// AsyncWrite-based downlink.
func TestGnetTcpEchoWithSticky(t *testing.T) {
	port := 36000 + int(time.Now().UnixNano()%1000)

	handler := &echoHandler{}
	svr := NewTcpServer(sharedstruct.ByteLenOfCSPacketHeader(), sharedstruct.ByteLenOfCSPacketBody, handler)
	handler.svr = svr
	if err := svr.Start("127.0.0.1", port); err != nil {
		t.Fatalf("gnet start failed: %v", err)
	}
	defer svr.Stop()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	frame1 := buildCSFrame(0x00020001, []byte("hello-gnet"))
	frame2 := buildCSFrame(0x00020003, []byte("second-frame-with-longer-body"))

	// 两帧一次性写入：验证事件循环内的 codec 正确拆包。
	sticky := append(append([]byte{}, frame1...), frame2...)
	if _, err := conn.Write(sticky); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	recv := make([]byte, len(sticky))
	if _, err := io.ReadFull(conn, recv); err != nil {
		t.Fatalf("read echo failed: %v", err)
	}
	if string(recv[:len(frame1)]) != string(frame1) {
		t.Fatal("echo frame1 mismatch")
	}
	if string(recv[len(frame1):]) != string(frame2) {
		t.Fatal("echo frame2 mismatch")
	}
	if got := handler.packets.Load(); got != 2 {
		t.Fatalf("expected 2 parsed packets, got %d", got)
	}
	if handler.opened.Load() != 1 {
		t.Fatalf("expected 1 opened conn, got %d", handler.opened.Load())
	}
}

// TestGnetTcpInvalidHeaderClosesConn verifies oversized body length closes
// the connection inside the codec.
func TestGnetTcpInvalidHeaderClosesConn(t *testing.T) {
	port := 37000 + int(time.Now().UnixNano()%1000)

	handler := &echoHandler{}
	svr := NewTcpServer(sharedstruct.ByteLenOfCSPacketHeader(), sharedstruct.ByteLenOfCSPacketBody, handler)
	handler.svr = svr
	if err := svr.Start("127.0.0.1", port); err != nil {
		t.Fatalf("gnet start failed: %v", err)
	}
	defer svr.Stop()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	bad := sharedstruct.CSPacketHeader{Uid: 1, Cmd: 1, BodyLen: 5 * 1024 * 1024}
	if _, err := conn.Write(bad.ToBytes()); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected connection close after invalid header")
	}
	if got := handler.packets.Load(); got != 0 {
		t.Fatalf("expected no parsed packets, got %d", got)
	}
}
