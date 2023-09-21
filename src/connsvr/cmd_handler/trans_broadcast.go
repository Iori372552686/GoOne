package cmd_handler

import (
	"GoOne/lib/api/cmd_handler"
	"GoOne/lib/api/sharedstruct"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/connsvr/globals"
)

type Broadcast struct{}

func (h *Broadcast) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.ConnBroadcastReq{}
	//rsp := &g1_protocol.ConnBroadcastRsp{}

	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0
	for {
		csPacketHeader := sharedstruct.CSPacketHeader{
			Uid:     c.Uid(),
			Cmd:     req.Cmd,
			BodyLen: uint32(len(req.Body)),
		}
		globals.ConnTcpSvr.BroadcastByZone(0, csPacketHeader.ToBytes(), req.Body)
		break
	}

	//rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	//c.SendMsgBack(rsp)
	return ret
}
