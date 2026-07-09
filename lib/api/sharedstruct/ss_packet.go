package sharedstruct

import (
	"encoding/binary"
	"fmt"
	"time"
)

type SSPacket struct {
	Header SSPacketHeader
	Body   []byte
}

// 经过测试，结构体是以8字节为单位对齐的，要注意一下
//
// v2 起头部追加 trace/deadline 透传字段（TraceID/SpanID/DeadlineUnixMs）。
// 这是集群内部协议的破坏性变更：新旧节点头长不一致（86 vs 54），
// 需要整组同时发版，不支持滚动混布。
type SSPacketHeader struct {
	SrcBusID uint32
	DstBusID uint32

	SrcTransID uint32
	DstTransID uint32

	Uid uint64

	RouterID uint64

	Cmd  uint32
	Zone uint32

	Ip   uint32
	Flag uint32

	BodyLen uint32
	CmdSeq  uint16 // Request时+1，Response时不变。用以标识收到的Response是对应哪个发出的Request

	// TraceID/SpanID 跨进程透传调用链标识；全零表示未启用。
	// 响应包通过复制请求头自然继承 trace（见 router.SendMsgBack）。
	TraceID [16]byte
	SpanID  [8]byte
	// DeadlineUnixMs 是本次请求的绝对截止时间（Unix 毫秒），0 表示无截止。
	// 接收端可据此丢弃已超期的请求，实现级联超时。
	DeadlineUnixMs int64
}

// ByteLenOfSSPacketHeader is the v2 wire size (54 legacy + 32 trace ext).
func ByteLenOfSSPacketHeader() int {
	return 86
}

// HasTrace reports whether the header carries a propagated trace id.
func (h *SSPacketHeader) HasTrace() bool {
	for _, b := range h.TraceID {
		if b != 0 {
			return true
		}
	}
	return false
}

func (h *SSPacketHeader) To(b []byte) error {
	if len(b) < ByteLenOfSSPacketHeader() {
		return fmt.Errorf("buffer is too small {bufSize:%v, expect:%v}", len(b), ByteLenOfSSPacketHeader())
	}

	pos := 0
	binary.BigEndian.PutUint32(b[pos:], h.SrcBusID)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.DstBusID)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.SrcTransID)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.DstTransID)
	pos += 4
	binary.BigEndian.PutUint64(b[pos:], h.Uid)
	pos += 8
	binary.BigEndian.PutUint64(b[pos:], h.RouterID)
	pos += 8
	binary.BigEndian.PutUint32(b[pos:], h.Cmd)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Zone)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Ip)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Flag)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.BodyLen)
	pos += 4
	binary.BigEndian.PutUint16(b[pos:], h.CmdSeq)
	pos += 2
	copy(b[pos:], h.TraceID[:])
	pos += 16
	copy(b[pos:], h.SpanID[:])
	pos += 8
	binary.BigEndian.PutUint64(b[pos:], uint64(h.DeadlineUnixMs))
	pos += 8

	return nil
}

func (h *SSPacketHeader) From(b []byte) error {
	if len(b) < ByteLenOfSSPacketHeader() {
		return fmt.Errorf("buffer is too small {bufSize:%v, expect:%v}", len(b), ByteLenOfSSPacketHeader())
	}

	pos := 0
	h.SrcBusID = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.DstBusID = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.SrcTransID = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.DstTransID = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Uid = binary.BigEndian.Uint64(b[pos:])
	pos += 8
	h.RouterID = binary.BigEndian.Uint64(b[pos:])
	pos += 8
	h.Cmd = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Zone = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Ip = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Flag = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.BodyLen = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.CmdSeq = binary.BigEndian.Uint16(b[pos:])
	pos += 2
	copy(h.TraceID[:], b[pos:pos+16])
	pos += 16
	copy(h.SpanID[:], b[pos:pos+8])
	pos += 8
	h.DeadlineUnixMs = int64(binary.BigEndian.Uint64(b[pos:]))
	pos += 8

	return nil
}

func (h *SSPacketHeader) ToBytes() []byte {
	bytes := make([]byte, ByteLenOfSSPacketHeader())
	h.To(bytes)
	return bytes
}

func (h *SSPacket) SendToChan(ch chan *SSPacket, timeout time.Duration) bool {
	// Fast path: channel has room, no timer allocation.
	select {
	case ch <- h:
		return true
	default:
	}

	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case ch <- h:
		return true
	case <-t.C:
		return false
	}
}
