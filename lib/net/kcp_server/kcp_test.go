package kcp_server

import (
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	kcp "github.com/xtaci/kcp-go/v5"
)

type testPacketHandler struct {
	svr     *KcpPacketSvr
	packets atomic.Int64
}

func (h *testPacketHandler) OnConn(conn net.Conn) {}

// OnPacket echoes every parsed CSPacket back through the write goroutine,
// covering frame split + WriteData + pooled buffers in one round trip.
func (h *testPacketHandler) OnPacket(conn net.Conn, data []byte) {
	h.packets.Add(1)
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	_ = h.svr.WriteData(conn, data[:headerLen], data[headerLen:])
}

func (h *testPacketHandler) OnClose(conn net.Conn) {}

func buildCSFrame(cmd uint32, body []byte) []byte {
	header := sharedstruct.CSPacketHeader{
		Uid:     10001,
		Cmd:     cmd,
		BodyLen: uint32(len(body)),
	}
	return append(header.ToBytes(), body...)
}

// TestKcpPacketEchoWithSticky verifies CSPacket framing over KCP: two frames
// written in a single kcp write (sticky packets) must be parsed as two
// packets and echoed back intact.
func TestKcpPacketEchoWithSticky(t *testing.T) {
	port := 38000 + int(time.Now().UnixNano()%1000)

	svr := &KcpPacketSvr{}
	handler := &testPacketHandler{svr: svr}
	packetInfo := KcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}
	if err := svr.InitAndRun("127.0.0.1", port, packetInfo, handler); err != nil {
		t.Fatalf("kcp server init failed: %v", err)
	}

	sess, err := kcp.DialWithOptions(net.JoinHostPort("127.0.0.1", itoa(port)), nil, 0, 0)
	if err != nil {
		t.Fatalf("dial kcp failed: %v", err)
	}
	defer sess.Close()
	sess.SetStreamMode(true)
	_ = sess.SetDeadline(time.Now().Add(10 * time.Second))

	frame1 := buildCSFrame(0x00020001, []byte("hello-kcp"))
	frame2 := buildCSFrame(0x00020003, []byte("second-frame-with-longer-body"))

	// 两帧一次性写入：验证服务端按 CSPacket 长度正确拆包。
	sticky := append(append([]byte{}, frame1...), frame2...)
	if _, err := sess.Write(sticky); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// 读回两帧 echo。
	recv := make([]byte, len(sticky))
	if _, err := io.ReadFull(sess, recv); err != nil {
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
}

// TestKcpInvalidHeaderClosesConn verifies that a body length exceeding the
// limit closes the connection instead of hanging the reassembly buffer.
func TestKcpInvalidHeaderClosesConn(t *testing.T) {
	port := 39000 + int(time.Now().UnixNano()%1000)

	svr := &KcpPacketSvr{}
	handler := &testPacketHandler{svr: svr}
	packetInfo := KcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}
	if err := svr.InitAndRun("127.0.0.1", port, packetInfo, handler); err != nil {
		t.Fatalf("kcp server init failed: %v", err)
	}

	sess, err := kcp.DialWithOptions(net.JoinHostPort("127.0.0.1", itoa(port)), nil, 0, 0)
	if err != nil {
		t.Fatalf("dial kcp failed: %v", err)
	}
	defer sess.Close()
	sess.SetStreamMode(true)
	_ = sess.SetDeadline(time.Now().Add(5 * time.Second))

	// BodyLen 超过 4MB 上限 → 服务端应判定非法并关闭连接。
	bad := sharedstruct.CSPacketHeader{Uid: 1, Cmd: 1, BodyLen: 5 * 1024 * 1024}
	if _, err := sess.Write(bad.ToBytes()); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	buf := make([]byte, 1)
	if _, err := sess.Read(buf); err == nil {
		t.Fatal("expected connection close after invalid header")
	}
	if got := handler.packets.Load(); got != 0 {
		t.Fatalf("expected no parsed packets, got %d", got)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
