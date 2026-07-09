package sharedstruct

import (
	"encoding/binary"
	"encoding/hex"
	"math/rand/v2"
	"time"
)

// TraceContext carries the cross-process trace identity and deadline that a
// caller stamps into the SSPacketHeader of an outbound request.
type TraceContext struct {
	TraceID        [16]byte
	SpanID         [8]byte
	DeadlineUnixMs int64 // 0 = no deadline
}

// NewTraceID returns a random 16-byte trace id (non-cryptographic; used for
// log correlation, not security).
func NewTraceID() [16]byte {
	var id [16]byte
	binary.BigEndian.PutUint64(id[0:8], rand.Uint64())
	binary.BigEndian.PutUint64(id[8:16], rand.Uint64())
	return id
}

// NewSpanID returns a random 8-byte span id.
func NewSpanID() [8]byte {
	var id [8]byte
	binary.BigEndian.PutUint64(id[:], rand.Uint64())
	return id
}

// ApplyTo stamps the trace context into a packet header.
func (tc TraceContext) ApplyTo(h *SSPacketHeader) {
	h.TraceID = tc.TraceID
	h.SpanID = tc.SpanID
	h.DeadlineUnixMs = tc.DeadlineUnixMs
}

// TraceIDHex returns the lowercase hex form of the header's trace id,
// or "" when no trace is carried.
func (h *SSPacketHeader) TraceIDHex() string {
	if !h.HasTrace() {
		return ""
	}
	return hex.EncodeToString(h.TraceID[:])
}

// SpanIDHex returns the lowercase hex form of the header's span id.
func (h *SSPacketHeader) SpanIDHex() string {
	return hex.EncodeToString(h.SpanID[:])
}

// Deadline converts DeadlineUnixMs into (time.Time, ok) form.
func (h *SSPacketHeader) Deadline() (time.Time, bool) {
	if h.DeadlineUnixMs <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(h.DeadlineUnixMs), true
}

// DeadlineExceeded reports whether the request carried a deadline that has
// already passed at nowMs.
func (h *SSPacketHeader) DeadlineExceeded(nowMs int64) bool {
	return h.DeadlineUnixMs > 0 && nowMs > h.DeadlineUnixMs
}
