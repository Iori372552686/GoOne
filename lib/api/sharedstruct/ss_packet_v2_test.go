package sharedstruct

import (
	"testing"
	"time"
)

func TestSSPacketHeaderV2RoundTrip(t *testing.T) {
	h := SSPacketHeader{
		SrcBusID:       0x01010101,
		DstBusID:       0x01010201,
		SrcTransID:     7,
		DstTransID:     8,
		Uid:            10001,
		RouterID:       77,
		Cmd:            0x00020001,
		Zone:           1,
		Ip:             0x7F000001,
		Flag:           12345,
		BodyLen:        128,
		CmdSeq:         3,
		TraceID:        NewTraceID(),
		SpanID:         NewSpanID(),
		DeadlineUnixMs: time.Now().Add(3 * time.Second).UnixMilli(),
	}

	buf := h.ToBytes()
	if len(buf) != ByteLenOfSSPacketHeader() {
		t.Fatalf("encoded len = %d, want %d", len(buf), ByteLenOfSSPacketHeader())
	}

	var out SSPacketHeader
	if err := out.From(buf); err != nil {
		t.Fatalf("From failed: %v", err)
	}
	if out != h {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, h)
	}
}

func TestSSPacketHeaderTraceHelpers(t *testing.T) {
	var h SSPacketHeader
	if h.HasTrace() {
		t.Fatal("zero header must not report a trace")
	}
	if h.TraceIDHex() != "" {
		t.Fatal("zero header TraceIDHex must be empty")
	}
	if _, ok := h.Deadline(); ok {
		t.Fatal("zero header must not report a deadline")
	}
	if h.DeadlineExceeded(time.Now().UnixMilli()) {
		t.Fatal("zero deadline must never be exceeded")
	}

	tc := TraceContext{
		TraceID:        NewTraceID(),
		SpanID:         NewSpanID(),
		DeadlineUnixMs: time.Now().Add(-time.Second).UnixMilli(), // already past
	}
	tc.ApplyTo(&h)

	if !h.HasTrace() {
		t.Fatal("header must report trace after ApplyTo")
	}
	if len(h.TraceIDHex()) != 32 || len(h.SpanIDHex()) != 16 {
		t.Fatalf("unexpected hex lengths: trace=%d span=%d", len(h.TraceIDHex()), len(h.SpanIDHex()))
	}
	if !h.DeadlineExceeded(time.Now().UnixMilli()) {
		t.Fatal("past deadline must be reported as exceeded")
	}
}

func TestNewTraceIDUniqueness(t *testing.T) {
	seen := map[[16]byte]bool{}
	for i := 0; i < 1000; i++ {
		id := NewTraceID()
		if seen[id] {
			t.Fatal("duplicate trace id generated")
		}
		seen[id] = true
	}
}
