// Package wire contains the shared wire-format helpers used by every bus
// driver: the 12-byte bus frame header, pooled frame assembly and bounded
// channel operations. It is internal to lib/service/bus/... — business code
// must depend on the bus.IBus abstraction instead.
package wire

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// PassCode is the magic value carried by every bus frame.
const PassCode = 0xFEED

// Header is the bus wire header (12 bytes, big endian).
type Header struct {
	Version  uint16
	PassCode uint16
	SrcBusID uint32
	DstBusID uint32
}

// HeaderLen returns the encoded size of Header.
func HeaderLen() int {
	return 12
}

func (h *Header) From(b []byte) {
	h.Version = binary.BigEndian.Uint16(b[0:])
	h.PassCode = binary.BigEndian.Uint16(b[2:])
	h.SrcBusID = binary.BigEndian.Uint32(b[4:])
	h.DstBusID = binary.BigEndian.Uint32(b[8:])
}

func (h *Header) To(b []byte) {
	binary.BigEndian.PutUint16(b[0:], h.Version)
	binary.BigEndian.PutUint16(b[2:], h.PassCode)
	binary.BigEndian.PutUint32(b[4:], h.SrcBusID)
	binary.BigEndian.PutUint32(b[8:], h.DstBusID)
}

// OutMsg is an assembled outbound frame queued to a driver's publish loop.
type OutMsg struct {
	BusID  uint32
	Topics string
	Data   []byte
}

// CalcQueueName maps a bus id to its queue/subject/topic name.
func CalcQueueName(busId uint32) string {
	return "bus_" + fmt.Sprintf("%x", busId)
}

// SleepOrStop waits for d, returning false immediately when stopCh closes.
// Unlike a bare time.After in a select, the timer is always released.
func SleepOrStop(stopCh <-chan struct{}, d time.Duration) bool {
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

// SendToMsgChan enqueues msg with a bounded wait.
// The fast path avoids the timer allocation entirely.
func SendToMsgChan(ch chan OutMsg, msg OutMsg, timeout time.Duration) bool {
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

// GetFrameBuf returns a length-n buffer, reusing pooled memory when possible.
func GetFrameBuf(n int) []byte {
	bp := frameBufPool.Get().(*[]byte)
	if cap(*bp) < n {
		frameBufPool.Put(bp)
		return make([]byte, n)
	}
	return (*bp)[:n]
}

// PutFrameBuf recycles a frame buffer. It must only be called after the MQ
// client has finished with the data (synchronous publish returned).
func PutFrameBuf(b []byte) {
	if cap(b) == 0 || cap(b) > maxPooledFrameSize {
		return
	}
	b = b[:0]
	frameBufPool.Put(&b)
}

// BuildFrame assembles the wire frame (bus header + data1 + data2) in a
// pooled buffer. The publish loop must call PutFrameBuf once the underlying
// MQ client's synchronous publish has returned.
func BuildFrame(srcBusId, dstBusId uint32, data1, data2 []byte) []byte {
	header := Header{
		Version:  0,
		PassCode: PassCode,
		SrcBusID: srcBusId,
		DstBusID: dstBusId,
	}

	data := GetFrameBuf(HeaderLen() + len(data1) + len(data2))
	header.To(data)
	pos := HeaderLen()
	copy(data[pos:], data1)
	pos += len(data1)
	copy(data[pos:], data2)
	return data
}
