package tester_util

import (
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
	"net"
	"testing"
)

const (
	SERVER_ADDR = "192.168.50.251:11001"
	UID         = 0
)

type Session struct {
	ServerAddr string
	Uid        uint64

	t    *testing.T
	conn net.Conn
}

func NewSession(t *testing.T) *Session {
	if t == nil {
		return nil
	}

	s := &Session{}
	s.ServerAddr = SERVER_ADDR
	s.Uid = UID

	s.t = t
	return s
}

func (s *Session) Open() error {
	var err error
	s.conn, err = net.Dial("tcp", s.ServerAddr)
	if err != nil {
		s.t.Error("[err]net.Dial: ", err)
		return err
	}
	s.t.Log("[ok]net.Dial")
	return nil
}

func (s *Session) Close() {
	if nil != s.conn {
		s.conn.Close()
	}
}

func (s *Session) Login() error {
	req := &g1_protocol.LoginReq{
		Account:   "fsdfsdfe",
		LoginType: "guest",
		ChannelId: 1,
	}
	err := s.SendCmd(uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), req)
	if err != nil {
		return err
	}

	rsp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{}}
	err = s.WaitTillCmd(uint32(g1_protocol.CMD_MAIN_LOGIN_RSP), rsp)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) Logout() error {
	reqLogout := g1_protocol.LogoutReq{}
	err := s.SendCmd(uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ), &reqLogout)
	if err != nil {
		return err
	}

	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{}}
	err = s.WaitTillCmd(uint32(g1_protocol.CMD_MAIN_LOGOUT_RSP), rsp)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) OpenAndLogin() error {
	err := s.Open()
	if err != nil {
		return err
	}
	return s.Login()
}

func (s *Session) LogoutAndClose() {
	if s.conn == nil {
		return
	}
	s.Logout()
	s.Close()
}

func (s *Session) Send(bytes []byte) {
	s.conn.Write(bytes)
}

func (s *Session) SendCmd(cmd uint32, req proto.Message) error {
	err := SendCmd(s.conn, s.Uid, cmd, req)
	if err != nil {
		s.t.Errorf("[err]Failed to SendCmd {err:%v, cmd:0x%x, req:%v}", err, cmd, req)
		return err
	}
	s.t.Logf("SentCmd: {cmd:0x%x, req:%v}", cmd, req)
	return nil
}

func (s *Session) WaitTillCmd(cmd uint32, rsp proto.Message) error {
	err := WaitTillCmd(s.conn, cmd, rsp)
	if err != nil {
		s.t.Errorf("[err]Failed to WaitTillCmd {err:%v, cmd:0x%x}", err, cmd)
		return err
	}
	s.t.Logf("RecvCmd: {cmd:0x%x, rsp:%v}", cmd, rsp)
	return nil
}
