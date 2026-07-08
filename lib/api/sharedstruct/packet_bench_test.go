package sharedstruct

import (
	"testing"
)

// Baseline benchmarks for packet header encode/decode. These are the
// per-message hot-path costs on both gateway and bus paths; see
// docs/optimization_roadmap.md phase 1.3 (header encoding pooling).

func BenchmarkSSPacketHeaderToBytes(b *testing.B) {
	h := &SSPacketHeader{
		SrcBusID: 0x01010101,
		DstBusID: 0x01010201,
		Uid:      10001,
		RouterID: 77,
		Cmd:      0x00020001,
		BodyLen:  128,
		CmdSeq:   1,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ToBytes()
	}
}

func BenchmarkSSPacketHeaderTo(b *testing.B) {
	h := &SSPacketHeader{
		SrcBusID: 0x01010101,
		DstBusID: 0x01010201,
		Uid:      10001,
		RouterID: 77,
		Cmd:      0x00020001,
		BodyLen:  128,
		CmdSeq:   1,
	}
	buf := make([]byte, ByteLenOfSSPacketHeader())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.To(buf)
	}
}

func BenchmarkSSPacketHeaderFrom(b *testing.B) {
	h := &SSPacketHeader{Uid: 10001, Cmd: 0x00020001, BodyLen: 128}
	buf := h.ToBytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out SSPacketHeader
		_ = out.From(buf)
	}
}

func BenchmarkCSPacketHeaderToBytes(b *testing.B) {
	h := &CSPacketHeader{
		Uid:     10001,
		Cmd:     0x00020001,
		BodyLen: 128,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.ToBytes()
	}
}

func BenchmarkCSPacketHeaderFrom(b *testing.B) {
	h := &CSPacketHeader{Uid: 10001, Cmd: 0x00020001, BodyLen: 128}
	buf := h.ToBytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out CSPacketHeader
		out.From(buf)
	}
}
