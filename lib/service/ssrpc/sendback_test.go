package ssrpc

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

type fakePB struct{}

func (f *fakePB) Reset()         {}
func (f *fakePB) String() string { return "fake" }
func (f *fakePB) ProtoMessage()  {}

type backerOnly struct {
	called int
}

func (b *backerOnly) SendMsgBack(pbMsg proto.Message) {
	b.called++
}

type backerWithCmd struct {
	calledWithCmd int
	lastCmd       g1_protocol.CMD
}

func (b *backerWithCmd) SendMsgBack(pbMsg proto.Message) {
	// not expected in this test
}

func (b *backerWithCmd) SendMsgBackWithCmd(cmd g1_protocol.CMD, pbMsg proto.Message) {
	b.calledWithCmd++
	b.lastCmd = cmd
}

func TestSendMsgBackWithCmd_Fallback(t *testing.T) {
	var msg fakePB
	b := &backerOnly{}
	SendMsgBackWithCmd(b, g1_protocol.CMD(123), &msg)
	if b.called != 1 {
		t.Fatalf("expected SendMsgBack called once, got %d", b.called)
	}
}

func TestSendMsgBackWithCmd_Preferred(t *testing.T) {
	var msg fakePB
	b := &backerWithCmd{}
	SendMsgBackWithCmd(b, g1_protocol.CMD(456), &msg)
	if b.calledWithCmd != 1 || b.lastCmd != 456 {
		t.Fatalf("expected SendMsgBackWithCmd called with 456, got cnt=%d cmd=%v", b.calledWithCmd, b.lastCmd)
	}
}


