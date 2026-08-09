package ssrpc

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
// SvrTypeFromCmd tests
// ---------------------------------------------------------------------------

func TestSvrTypeFromCmd(t *testing.T) {
	tests := []struct {
		cmd      g1_protocol.CMD
		wantType uint32
	}{
		// CMD = 0x01020001 => svrType = 0x02 (bits [23:16])
		{g1_protocol.CMD(0x01020001), 0x02},
		// CMD = 0x00030000 => svrType = 0x03
		{g1_protocol.CMD(0x00030000), 0x03},
		// CMD = 0x00000000 => svrType = 0
		{g1_protocol.CMD(0), 0},
		// CMD = 0x00FF0000 => svrType = 0xFF
		{g1_protocol.CMD(0x00FF0000), 0xFF},
	}

	for _, tt := range tests {
		got := SvrTypeFromCmd(tt.cmd)
		if got != tt.wantType {
			t.Errorf("SvrTypeFromCmd(0x%X) = %d, want %d", uint32(tt.cmd), got, tt.wantType)
		}
	}
}

// ---------------------------------------------------------------------------
// mockCallContext -- captures CallMsgBySvrType / SendMsgByServerType calls
// ---------------------------------------------------------------------------

type mockCallContext struct {
	fakeIContext // embed for logging stubs, etc.

	calledSvrType uint32
	calledCmd     g1_protocol.CMD
	calledReq     proto.Message
	calledRsp     proto.Message
	calledRouter  uint64
	callErr       error // error to return from Call*
	sendCalled    bool
	routerCalled  bool
}

func (m *mockCallContext) CallMsgBySvrType(svrType uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	m.calledSvrType = svrType
	m.calledCmd = cmd
	m.calledReq = req
	m.calledRsp = rsp
	return m.callErr
}

func (m *mockCallContext) SendMsgByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error {
	m.calledSvrType = svrType
	m.calledCmd = cmd
	m.calledReq = req
	m.sendCalled = true
	return m.callErr
}

func (m *mockCallContext) CallMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	m.calledSvrType = svrType
	m.calledCmd = cmd
	m.calledReq = req
	m.calledRsp = rsp
	m.calledRouter = routerId
	m.routerCalled = true
	return m.callErr
}

func (m *mockCallContext) SendMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message) error {
	m.calledSvrType = svrType
	m.calledCmd = cmd
	m.calledReq = req
	m.calledRouter = routerId
	m.sendCalled = true
	return m.callErr
}

// ---------------------------------------------------------------------------
// CallByCmd tests
// ---------------------------------------------------------------------------

func TestCallByCmd_DerivesSvrType(t *testing.T) {
	mock := &mockCallContext{}
	cmd := g1_protocol.CMD(0x01020001) // svrType = 0x02
	req := &fakePB{}
	rsp := &fakePB{}

	err := CallByCmd(mock, cmd, req, rsp)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mock.calledSvrType != 0x02 {
		t.Fatalf("svrType = %d, want 2", mock.calledSvrType)
	}
	if mock.calledCmd != cmd {
		t.Fatalf("cmd = %v, want %v", mock.calledCmd, cmd)
	}
	if mock.calledReq != req {
		t.Fatal("req not passed through")
	}
}

func TestCallByCmd_PropagatesError(t *testing.T) {
	mock := &mockCallContext{callErr: E(g1_protocol.ErrorCode_ERR_TIMEOUT, "timeout")}
	err := CallByCmd(mock, g1_protocol.CMD(0x01020001), &fakePB{}, &fakePB{})
	if err == nil {
		t.Fatal("expected error")
	}
	if ToErrorCode(err) != g1_protocol.ErrorCode_ERR_TIMEOUT {
		t.Fatalf("expected TIMEOUT, got %v", ToErrorCode(err))
	}
}

// ---------------------------------------------------------------------------
// SendByCmd tests
// ---------------------------------------------------------------------------

func TestSendByCmd_FireAndForget(t *testing.T) {
	mock := &mockCallContext{}
	cmd := g1_protocol.CMD(0x00030000) // svrType = 0x03
	req := &fakePB{}

	err := SendByCmd(mock, cmd, req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !mock.sendCalled {
		t.Fatal("SendMsgByServerType not called")
	}
	if mock.calledSvrType != 0x03 {
		t.Fatalf("svrType = %d, want 3", mock.calledSvrType)
	}
}

// ---------------------------------------------------------------------------
// CallByCmdWithRouter tests
// ---------------------------------------------------------------------------

func TestCallByCmdWithRouter_UsesRouterId(t *testing.T) {
	mock := &mockCallContext{}
	cmd := g1_protocol.CMD(0x01020001) // svrType = 0x02
	routerId := uint64(99999)

	err := CallByCmdWithRouter(mock, routerId, cmd, &fakePB{}, &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !mock.routerCalled {
		t.Fatal("CallMsgByRouter not called")
	}
	if mock.calledRouter != routerId {
		t.Fatalf("routerId = %d, want %d", mock.calledRouter, routerId)
	}
	if mock.calledSvrType != 0x02 {
		t.Fatalf("svrType = %d, want 2", mock.calledSvrType)
	}
}

// ---------------------------------------------------------------------------
// SendByCmdWithRouter tests
// ---------------------------------------------------------------------------

func TestSendByCmdWithRouter(t *testing.T) {
	mock := &mockCallContext{}
	cmd := g1_protocol.CMD(0x00040000) // svrType = 0x04
	routerId := uint64(12345)

	err := SendByCmdWithRouter(mock, routerId, cmd, &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !mock.sendCalled {
		t.Fatal("SendMsgByRouter not called")
	}
	if mock.calledRouter != routerId {
		t.Fatalf("routerId = %d, want %d", mock.calledRouter, routerId)
	}
	if mock.calledSvrType != 0x04 {
		t.Fatalf("svrType = %d, want 4", mock.calledSvrType)
	}
}
