package router

import (
	"bytes"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
)

type fakeBus struct {
	selfBusID uint32

	sendCalls  int
	closeCalls int
	lastDst    uint32
	lastData1  []byte
	lastData2  []byte
}

func (b *fakeBus) SelfBusId() uint32 {
	return b.selfBusID
}

func (b *fakeBus) Send(dstBusId uint32, data1 []byte, data2 []byte) error {
	b.sendCalls++
	b.lastDst = dstBusId
	b.lastData1 = append([]byte(nil), data1...)
	b.lastData2 = append([]byte(nil), data2...)
	return nil
}

func (b *fakeBus) SetReceiver(_ bus.MsgHandler) {}

func (b *fakeBus) Healthy() bool { return true }

func (b *fakeBus) Close() error {
	b.closeCalls++
	return nil
}

// newTestRouter builds an isolated Router with a fake bus injected,
// demonstrating the struct-based API for tests (no global state involved).
func newTestRouter(fb *fakeBus, cb CbOnRecvSSPacket) *Router {
	r := New()
	r.busImpl = fb
	r.cbOnRecvSSPacket = cb
	return r
}

func TestRouterCloseClosesBusAndClearsState(t *testing.T) {
	fb := &fakeBus{selfBusID: 0x01020304}
	r := newTestRouter(fb, func(packet *sharedstruct.SSPacket) {})

	if err := r.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if fb.closeCalls != 1 {
		t.Fatalf("expected bus Close to be called once, got %d", fb.closeCalls)
	}
	if r.busImpl != nil {
		t.Fatal("expected router busImpl to be cleared")
	}
	if r.cbOnRecvSSPacket != nil {
		t.Fatal("expected router callback to be cleared")
	}

	// Close is idempotent.
	if err := r.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	if fb.closeCalls != 1 {
		t.Fatalf("expected bus Close to remain 1, got %d", fb.closeCalls)
	}
}

func TestRouterReadyCheck(t *testing.T) {
	fb := &fakeBus{selfBusID: 0x01020304}
	r := newTestRouter(fb, nil)

	if err := r.ReadyCheck(); err != nil {
		t.Fatalf("expected ready with healthy bus, got %v", err)
	}

	r.BeginShutdown()
	if err := r.ReadyCheck(); err == nil {
		t.Fatal("expected not-ready while shutting down")
	}
}

func TestSendMsg_LocalBusShortCircuits(t *testing.T) {
	fb := &fakeBus{selfBusID: 0x01020304}

	var gotPacket *sharedstruct.SSPacket
	r := newTestRouter(fb, func(packet *sharedstruct.SSPacket) {
		gotPacket = packet
	})

	header := &sharedstruct.SSPacketHeader{
		SrcBusID: fb.selfBusID,
		DstBusID: fb.selfBusID,
		Uid:      1001,
		RouterID: 77,
		Cmd:      99,
		BodyLen:  3,
	}
	body := []byte{1, 2, 3}

	if err := r.SendMsg(header, body); err != nil {
		t.Fatalf("SendMsg returned error: %v", err)
	}
	if fb.sendCalls != 0 {
		t.Fatalf("expected local send to bypass bus, got %d bus sends", fb.sendCalls)
	}
	if gotPacket == nil {
		t.Fatal("expected local send to invoke receive callback")
	}
	if !bytes.Equal(gotPacket.Body, []byte{1, 2, 3}) {
		t.Fatalf("unexpected local packet body: %v", gotPacket.Body)
	}
	if gotPacket.Header.DstBusID != fb.selfBusID {
		t.Fatalf("unexpected local packet header: %+v", gotPacket.Header)
	}

	body[0] = 9
	header.DstBusID = 0x0A0B0C0D
	if gotPacket.Body[0] != 1 {
		t.Fatalf("expected local packet body to be copied, got %v", gotPacket.Body)
	}
	if gotPacket.Header.DstBusID != fb.selfBusID {
		t.Fatalf("expected local packet header to be copied, got %+v", gotPacket.Header)
	}
}

func TestSendMsg_RemoteBusUsesBusImpl(t *testing.T) {
	fb := &fakeBus{selfBusID: 0x01020304}

	callbackCalled := false
	r := newTestRouter(fb, func(packet *sharedstruct.SSPacket) {
		callbackCalled = true
	})

	header := &sharedstruct.SSPacketHeader{
		SrcBusID: fb.selfBusID,
		DstBusID: 0x05060708,
		Uid:      2002,
		RouterID: 88,
		Cmd:      101,
		BodyLen:  2,
	}
	body := []byte{4, 5}

	if err := r.SendMsg(header, body); err != nil {
		t.Fatalf("SendMsg returned error: %v", err)
	}
	if callbackCalled {
		t.Fatal("expected remote send not to invoke local callback")
	}
	if fb.sendCalls != 1 {
		t.Fatalf("expected remote send to use bus once, got %d", fb.sendCalls)
	}
	if fb.lastDst != header.DstBusID {
		t.Fatalf("unexpected bus dst: got %v want %v", fb.lastDst, header.DstBusID)
	}
	if !bytes.Equal(fb.lastData1, header.ToBytes()) {
		t.Fatalf("unexpected header payload sent to bus")
	}
	if !bytes.Equal(fb.lastData2, body) {
		t.Fatalf("unexpected body payload sent to bus: %v", fb.lastData2)
	}
}

func TestSendMsgByBusId_RejectsZeroBusId(t *testing.T) {
	fb := &fakeBus{selfBusID: 0x01020304}
	r := newTestRouter(fb, nil)

	if err := r.SendMsgByBusId(0, 0, 1, 1, 0, 0, 0, nil); err == nil {
		t.Fatal("expected error for zero bus id")
	}
	if fb.sendCalls != 0 {
		t.Fatalf("expected no bus send, got %d", fb.sendCalls)
	}
}

func TestPackageLevelAPIUsesDefaultRouter(t *testing.T) {
	// Package-level helpers must operate on Default(). Swap in a fake bus
	// and restore afterwards.
	old := defaultRouter
	t.Cleanup(func() { defaultRouter = old })

	fb := &fakeBus{selfBusID: 0x01020304}
	defaultRouter = newTestRouter(fb, nil)

	if got := SelfBusId(); got != fb.selfBusID {
		t.Fatalf("SelfBusId = %#x, want %#x", got, fb.selfBusID)
	}
	if Default() != defaultRouter {
		t.Fatal("Default() must return the package default instance")
	}
	if err := ReadyCheck(); err != nil {
		t.Fatalf("ReadyCheck via package API failed: %v", err)
	}
}
