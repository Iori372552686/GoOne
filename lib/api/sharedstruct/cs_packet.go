package sharedstruct

import (
	"encoding/binary"
)

type CSPacket struct {
	Header CSPacketHeader
	Body   []byte
}

// 注意这里的排列是考虑了内存对齐的情况，调整时请注意。
type CSPacketHeader struct {
	Version  uint16
	PassCode uint16
	Seq      uint32

	Uid uint64

	AppVersion uint32
	Cmd        uint32

	BodyLen uint32
}

func ByteLenOfCSPacketHeader() int {
	return 28
}

func ByteLenOfCSPacketBody(header []byte) int {
	return int(binary.BigEndian.Uint32(header[ByteLenOfCSPacketHeader()-4:]))
}

func (h *CSPacketHeader) From(b []byte) {
	pos := 0
	h.Version = binary.BigEndian.Uint16(b[pos:])
	pos += 2
	h.PassCode = binary.BigEndian.Uint16(b[pos:])
	pos += 2
	h.Seq = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Uid = binary.BigEndian.Uint64(b[pos:])
	pos += 8
	h.AppVersion = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.Cmd = binary.BigEndian.Uint32(b[pos:])
	pos += 4
	h.BodyLen = binary.BigEndian.Uint32(b[pos:])
	pos += 4
}

func (h *CSPacketHeader) To(b []byte) {
	pos := uintptr(0)
	binary.BigEndian.PutUint16(b[pos:], h.Version)
	pos += 2
	binary.BigEndian.PutUint16(b[pos:], h.PassCode)
	pos += 2
	binary.BigEndian.PutUint32(b[pos:], h.Seq)
	pos += 4
	binary.BigEndian.PutUint64(b[pos:], h.Uid)
	pos += 8
	binary.BigEndian.PutUint32(b[pos:], h.AppVersion)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.Cmd)
	pos += 4
	binary.BigEndian.PutUint32(b[pos:], h.BodyLen)
	pos += 4
}

func (h *CSPacketHeader) ToBytes() []byte {
	bytes := make([]byte, ByteLenOfCSPacketHeader())
	h.To(bytes)
	return bytes
}

func (h *CSPacketHeader) Size() int {
	return ByteLenOfCSPacketHeader()
}
