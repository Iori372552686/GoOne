package net_mgr

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	kcp "github.com/xtaci/kcp-go/v5"
)

// TestKcpGatewaySession covers the full GatewayServer contract on KCP:
// bind (UpdateClientByUid), downlink (SendByUid) and Kick.
func TestKcpGatewaySession(t *testing.T) {
	port := 35000 + int(time.Now().UnixNano()%1000)
	const uid = uint64(70001)

	gw := NewKcpSvr()
	// 网关回调：首包即绑定会话（真实路径由登录流程绑定）。
	err := gw.CreateKcpServer(port, func(conn net.Conn, data []byte) {
		gw.UpdateClientByUid(conn, uid, 1)
	})
	if err != nil {
		t.Fatalf("kcp gateway start failed: %v", err)
	}

	sess, err := kcp.DialWithOptions(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil, 0, 0)
	if err != nil {
		t.Fatalf("dial kcp failed: %v", err)
	}
	defer sess.Close()
	sess.SetStreamMode(true)
	_ = sess.SetDeadline(time.Now().Add(10 * time.Second))

	// 发送一帧触发绑定。
	bind := sharedstruct.CSPacketHeader{Uid: uid, Cmd: 0x00020001, BodyLen: 0}
	if _, err := sess.Write(bind.ToBytes()); err != nil {
		t.Fatalf("write bind frame failed: %v", err)
	}

	// 等待会话建立。
	deadline := time.Now().Add(5 * time.Second)
	for gw.GetClientByUid(uid) == nil {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for session bind")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 下行：SendByUid。
	down := sharedstruct.CSPacketHeader{Uid: uid, Cmd: 0x00020004, BodyLen: 5}
	var headerBuf [28]byte
	down.To(headerBuf[:])
	if err := gw.SendByUid(uid, headerBuf[:], []byte("hello")); err != nil {
		t.Fatalf("SendByUid failed: %v", err)
	}

	recv := make([]byte, 28+5)
	if _, err := readFull(sess, recv); err != nil {
		t.Fatalf("read downlink failed: %v", err)
	}
	var got sharedstruct.CSPacketHeader
	got.From(recv)
	if got.Cmd != 0x00020004 || string(recv[28:]) != "hello" {
		t.Fatalf("unexpected downlink frame: %+v body=%q", got, recv[28:])
	}

	// 踢人：应收到 CMD_SC_KICK_OUT 帧，随后连接关闭。
	gw.Kick(uid, g1_protocol.EKickOutReason_HEARTBEAT_TIMEOUT)

	kickHeader := make([]byte, 28)
	if _, err := readFull(sess, kickHeader); err != nil {
		t.Fatalf("read kick frame failed: %v", err)
	}
	var kick sharedstruct.CSPacketHeader
	kick.From(kickHeader)
	if kick.Cmd != uint32(g1_protocol.CMD_SC_KICK_OUT) {
		t.Fatalf("expected kickout cmd, got %#x", kick.Cmd)
	}
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
