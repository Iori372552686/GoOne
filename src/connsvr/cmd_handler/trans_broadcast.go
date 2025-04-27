package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func Broadcast(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.ConnBroadcastReq{}

	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	csPacketHeader := sharedstruct.CSPacketHeader{
		Uid:     c.Uid(),
		Cmd:     req.Cmd,
		BodyLen: uint32(len(req.Body)),
	}

	//globals.ConnTcpSvr.BroadcastByZone(0, csPacketHeader.ToBytes(), req.Body)
	globals.ConnWsSvr.BroadcastByZone(0, csPacketHeader.ToBytes(), req.Body)
	return g1_protocol.ErrorCode_ERR_OK
}
