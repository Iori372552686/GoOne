package bus

import (
	"encoding/binary"
	"fmt"
	"time"
)

// cb  handler
type MsgHandler func(srcBusID uint32, data []byte) error

// 需保证协程并发安全
type IBus interface {
	SelfBusId() uint32
	Send(dstBusId uint32, data1 []byte, data2 []byte) error

	// 默认规则：
	// 1. onRecvMsg由实现类的内部协程调用，且只会由一个协程调用。
	// 2. data的所有权，转交给onRecvMsg。
	// 如有例外，实现类需特殊说明。
	SetReceiver(onRecvMsg MsgHandler)
}

// -------------------------------- private --------------------------------

const (
	passCode = 0xFEED
)

type busPacket struct {
	Header busPacketHeader
	Body   []byte
}

type busPacketHeader struct {
	version  uint16
	passCode uint16
	srcBusId uint32
	dstBusId uint32
}

func byteLenOfBusPacketHeader() int {
	return 12
}

func (h *busPacketHeader) From(b []byte) {
	h.version = binary.BigEndian.Uint16(b[0:])
	h.passCode = binary.BigEndian.Uint16(b[2:])
	h.srcBusId = binary.BigEndian.Uint32(b[4:])
	h.dstBusId = binary.BigEndian.Uint32(b[8:])
}

func (h *busPacketHeader) To(b []byte) {
	binary.BigEndian.PutUint16(b[0:], h.version)
	binary.BigEndian.PutUint16(b[2:], h.passCode)
	binary.BigEndian.PutUint32(b[4:], h.srcBusId)
	binary.BigEndian.PutUint32(b[8:], h.dstBusId)
}

type outMsg struct {
	busId  uint32
	topics string
	data   []byte
}

func calcQueueName(busId uint32) string {
	return "bus_" + fmt.Sprintf("%x", busId)
}

func sendToMsgChan(ch chan outMsg, msg outMsg, timeout time.Duration) bool {
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case ch <- msg:
	case <-t.C:
		return false
	}

	return true
}
