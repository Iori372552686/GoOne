package bus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// cb  handler
type MsgHandler func(srcBusID uint32, data []byte) error

// 需保证协程并发安全
type IBus interface {
	SelfBusId() uint32
	Send(dstBusId uint32, data1 []byte, data2 []byte) error
	Close() error

	// Healthy 返回当前与 MQ 的连接是否就绪（用于 readyz 健康检查）。
	Healthy() bool

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

var ErrBusClosed = errors.New("bus is closed")

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

// ---- outbound frame buffer pool ----
//
// Every Send() builds a full wire frame (bus header + ss header + body).
// Frames are handed to the publish loop via chanOut and are no longer
// referenced after the underlying MQ client's synchronous publish returns,
// so they can be pooled to avoid one allocation per message.

const maxPooledFrameSize = 64 * 1024

var frameBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

// getFrameBuf returns a length-n buffer, reusing pooled memory when possible.
func getFrameBuf(n int) []byte {
	bp := frameBufPool.Get().(*[]byte)
	if cap(*bp) < n {
		frameBufPool.Put(bp)
		return make([]byte, n)
	}
	return (*bp)[:n]
}

// putFrameBuf recycles a frame buffer. It must only be called after the MQ
// client has finished with the data (synchronous publish returned).
func putFrameBuf(b []byte) {
	if cap(b) == 0 || cap(b) > maxPooledFrameSize {
		return
	}
	b = b[:0]
	frameBufPool.Put(&b)
}

// buildFrame assembles the wire frame (bus header + data1 + data2) in a
// pooled buffer. The publish loop must call putFrameBuf once the underlying
// MQ client's synchronous publish has returned.
func buildFrame(srcBusId, dstBusId uint32, data1, data2 []byte) []byte {
	header := busPacketHeader{
		version:  0,
		passCode: passCode,
		srcBusId: srcBusId,
		dstBusId: dstBusId,
	}

	data := getFrameBuf(byteLenOfBusPacketHeader() + len(data1) + len(data2))
	header.To(data)
	pos := byteLenOfBusPacketHeader()
	copy(data[pos:], data1)
	pos += len(data1)
	copy(data[pos:], data2)
	return data
}

// sleepOrStop waits for d, returning false immediately when stopCh closes.
// Unlike a bare time.After in a select, the timer is always released.
func sleepOrStop(stopCh <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-stopCh:
			return false
		default:
			return true
		}
	}

	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stopCh:
		return false
	case <-t.C:
		return true
	}
}

func sendToMsgChan(ch chan outMsg, msg outMsg, timeout time.Duration) bool {
	// Fast path: queue has room, no timer allocation.
	select {
	case ch <- msg:
		return true
	default:
	}

	t := time.NewTimer(timeout)
	defer t.Stop()

	select {
	case ch <- msg:
	case <-t.C:
		return false
	}

	return true
}
