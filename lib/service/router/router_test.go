package router

import (
	"bytes"
	"sync"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
)

type fakeBus struct {
	selfBusID uint32

	sendCalls int
	closeCalls int
	lastDst   uint32
	lastData1 []byte
	lastData2 []byte
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

func (b *fakeBus) Close() error {
	b.closeCalls++
	return nil
}

func TestRouterCloseClosesBusAndClearsState(t *testing.T) {
	oldBus := router.busImpl
	oldCb := router.cbOnRecvSSPacket
	t.Cleanup(func() {
		router.busImpl = oldBus
		router.cbOnRecvSSPacket = oldCb
		router.beginShutdownOnce = sync.Once{}
		router.closeOnce = sync.Once{}
	})

	fb := &fakeBus{selfBusID: 0x01020304}
	router.busImpl = fb
	router.cbOnRecvSSPacket = func(packet *sharedstruct.SSPacket) {}
	router.beginShutdownOnce = sync.Once{}
	router.closeOnce = sync.Once{}

	if err := Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if fb.closeCalls != 1 {
		t.Fatalf("expected bus Close to be called once, got %d", fb.closeCalls)
	}
	if router.busImpl != nil {
		t.Fatal("expected router busImpl to be cleared")
	}
	if router.cbOnRecvSSPacket != nil {
		t.Fatal("expected router callback to be cleared")
	}
}

func TestSendMsg_LocalBusShortCircuits(t *testing.T) {
	oldBus := router.busImpl
	oldCb := router.cbOnRecvSSPacket
	t.Cleanup(func() {
		router.busImpl = oldBus
		router.cbOnRecvSSPacket = oldCb
	})

	fb := &fakeBus{selfBusID: 0x01020304}
	router.busImpl = fb

	var gotPacket *sharedstruct.SSPacket
	router.cbOnRecvSSPacket = func(packet *sharedstruct.SSPacket) {
		gotPacket = packet
	}

	header := &sharedstruct.SSPacketHeader{
		SrcBusID: fb.selfBusID,
		DstBusID: fb.selfBusID,
		Uid:      1001,
		RouterID: 77,
		Cmd:      99,
		BodyLen:  3,
	}
	body := []byte{1, 2, 3}

	if err := SendMsg(header, body); err != nil {
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
	oldBus := router.busImpl
	oldCb := router.cbOnRecvSSPacket
	t.Cleanup(func() {
		router.busImpl = oldBus
		router.cbOnRecvSSPacket = oldCb
	})

	fb := &fakeBus{selfBusID: 0x01020304}
	router.busImpl = fb

	callbackCalled := false
	router.cbOnRecvSSPacket = func(packet *sharedstruct.SSPacket) {
		callbackCalled = true
	}

	header := &sharedstruct.SSPacketHeader{
		SrcBusID: fb.selfBusID,
		DstBusID: 0x05060708,
		Uid:      2002,
		RouterID: 88,
		Cmd:      101,
		BodyLen:  2,
	}
	body := []byte{4, 5}

	if err := SendMsg(header, body); err != nil {
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
