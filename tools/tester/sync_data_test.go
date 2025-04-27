package tester

import (
	"github.com/Iori372552686/GoOne/tools/tester/tester_util"
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
	"testing"
)

func TestSyncClientData(t *testing.T) {
	s := tester_util.NewSession(t)
	err := s.OpenAndLogin()
	if err != nil {
		return
	}
	defer s.LogoutAndClose()

	req := &g1_protocol.MallBuyPackageReq{
		ConfId: 8,
	}
	err = s.SendCmd(uint32(g1_protocol.CMD_MAIN_MALL_BUY_PACKAGE_REQ), req)
	if err != nil {
		return
	}

	msg := &g1_protocol.ScSyncUserData{}
	err = s.WaitTillCmd(uint32(g1_protocol.CMD_SC_SYNC_USER_DATA), msg)
	if err != nil {
		return
	}

	//rsp := &g1_protocol.BuyRsp{}
	//err = s.WaitTillCmd(uint32(g1_protocol.CMD_MAIN_BUY_RSP), rsp)
	//if err != nil {
	//	return
	//}
}
