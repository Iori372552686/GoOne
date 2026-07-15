package connsvr

import (
	"net"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
)

// TestHandleClientPacketRejectsUidZero 验证 uid==0 的客户端包被拒绝（不路由、不 panic）。
// 这是 pack_proc.go 的核心安全边界：首次连接未携带 uid 时直接丢弃。
func TestHandleClientPacketRejectsUidZero(t *testing.T) {
	// 构造一个合法的 CS 包，uid=0
	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      0, // 关键：uid=0 应被拒绝
		Cmd:      0x20000,
		BodyLen:  0,
	}
	headerBytes := header.ToBytes()

	// 用 pipe 模拟 net.Conn
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// handleClientPacket 对 uid==0 应直接 return（不走 gateway/router）
	// 传 nil gateway 验证：如果 uid==0 分支没生效，会 panic on nil gw
	handleClientPacket(nil, "tcp", client, headerBytes)
	// 到这里没 panic = 测试通过（uid==0 被正确拒绝）
}

// TestHandleClientPacketRejectsShortPacket 验证短包（< header len）被拒绝。
func TestHandleClientPacketRejectsShortPacket(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// 只传 10 字节，远小于 28 字节头
	handleClientPacket(nil, "tcp", client, make([]byte, 10))
	// 没 panic = 通过
}

// TestHandleClientPacketRejectsInnerCmd 验证内部 cmd（IsInnerCmd）被拒绝。
func TestHandleClientPacketRejectsInnerCmd(t *testing.T) {
	// 构造一个内部 cmd 的包（MsgTypeInCmd != 0 且 != 3）
	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      12345,
		// cmd = 0x21000, MsgTypeInCmd = (0x21000>>12)&0xf = 0x21&0xf = 1 → IsClientCmd=false → inner
		Cmd:     0x21000,
		BodyLen: 0,
	}
	headerBytes := header.ToBytes()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// 内部 cmd 应被拒绝（不走路由），传 nil gateway 验证不 panic
	handleClientPacket(nil, "tcp", client, headerBytes)
	// 没 panic = 通过
}
