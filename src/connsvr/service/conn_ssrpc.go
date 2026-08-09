package service

import (
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// ConnServiceImpl is the IDL-driven ssrpc implementation for connsvr internal RPCs.
type ConnServiceImpl struct{}

func (s *ConnServiceImpl) KickOut(ctx *ssrpc.Context, req *g1_protocol.ConnKickOutReq) (*g1_protocol.ConnKickOutRsp, error) {
	if ctx == nil || req == nil {
		return nil, nil
	}

	ctx.Infof("conn kickout reason=%v remote_addr=%s", req.GetReason(), req.GetRemoteAddr())
	globals.ConnWsSvr.KickByRemoteAddr(ctx.Uid(), req.GetReason(), req.GetRemoteAddr())
	return nil, nil
}

func (s *ConnServiceImpl) Broadcast(ctx *ssrpc.Context, req *g1_protocol.ConnBroadcastReq) (*g1_protocol.ConnBroadcastRsp, error) {
	csPacketHeader := sharedstruct.CSPacketHeader{
		Uid:     ctx.Uid(),
		Cmd:     req.Cmd,
		BodyLen: uint32(len(req.Body)),
	}
	globals.ConnWsSvr.BroadcastByZone(0, csPacketHeader.ToBytes(), req.Body)
	return &g1_protocol.ConnBroadcastRsp{}, nil
}
