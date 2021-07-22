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
type SSPacketHeader struct {
	SrcBusID	uint32
	DstBusID 	uint32

	SrcTransID 	uint32
	DstTransID 	uint32

	Uid 		uint64

	Cmd		uint32
	Zone 		uint32

	Ip 		uint32
	Flag 		uint32

	BodyLen 	uint32
	CmdSeq  	uint16	// Request时+1，Response时不变。用以标识收到的Response是对应哪个发出的Request
}


func ByteLenOfSSPacketHeader() int {
	return 46
}

func (h *SSPacketHeader) To(b []byte) error {
	if len(b) < ByteLenOfSSPacketHeader() {
		return fmt.Errorf("buffer is too small {bufSize:%v, expect:%v}", len(b), ByteLenOfSSPacketHeader())
	}

	pos := 0
	binary.BigEndian.PutUint32(b[pos:], h.SrcBusID); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.DstBusID); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.SrcTransID); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.DstTransID); pos += 4
	binary.BigEndian.PutUint64(b[pos:], h.Uid); pos += 8
	binary.BigEndian.PutUint32(b[pos:], h.Cmd); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Zone); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Ip); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Flag); pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.BodyLen); pos += 4
	binary.BigEndian.PutUint16(b[pos:], h.CmdSeq); pos += 2

	return nil
}


func (h *SSPacketHeader) From(b []byte) error {
	if len(b) < ByteLenOfSSPacketHeader() {
		return fmt.Errorf("buffer is too small {bufSize:%v, expect:%v}", len(b), ByteLenOfSSPacketHeader())
	}

	pos := 0
	h.SrcBusID = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.DstBusID = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.SrcTransID = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.DstTransID = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.Uid = binary.BigEndian.Uint64(b[pos:]); pos += 8
	h.Cmd = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.Zone = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.Ip = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.Flag = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.BodyLen = binary.BigEndian.Uint32(b[pos:]); pos += 4
	h.CmdSeq = binary.BigEndian.Uint16(b[pos:]); pos += 2

	return nil
}

func (h *SSPacketHeader) ToBytes() []byte {
	bytes := make([]byte, ByteLenOfSSPacketHeader())
	h.To(bytes)
	return bytes
}


func (h *SSPacket) SendToChan(ch chan *SSPacket, timeout time.Duration) bool {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case ch <- h: return true
	case <- t.C: return false
	}
}