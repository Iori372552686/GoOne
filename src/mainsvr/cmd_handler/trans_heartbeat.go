package cmd_handler

import (
	"GoOne/lib/api/cmd_handler"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/mainsvr/role"
)

type HeartBeat struct{}

func (t *HeartBeat) ProcessCmd(c cmd_handler.IContext, data []byte, myRole *role.Role) int {
	req := &g1_protocol.HeartBeatReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0
	now := myRole.Now()

	myRole.OnClientHeartbeat(now)

	rsp := &g1_protocol.HeartBeatRsp{}
	rsp.ClientNowMsInReq = req.ClientNowMs
	rsp.ServerNowMs = myRole.NowMs()
	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(rsp)

	return ret
}
