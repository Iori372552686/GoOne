package cmd_handler

import (
	`GoOne/lib/cmd_handler`
	`GoOne/lib/sharedstruct`
	g1_protocol `GoOne/protobuf/protocol`
	`GoOne/src/connsvr/globals`
)

type Broadcast struct {}
func (h *Broadcast) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.ConnBroadcastReq{}

	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0
	csPacketHeader := sharedstruct.CSPacketHeader{
			Uid: c.Uid(),
			Cmd: req.Cmd,
			BodyLen: uint32(len(req.Body)),
	}
	globals.ClientMgr.BroadcastByZone(0, csPacketHeader.ToBytes(), req.Body)

	return ret
}

