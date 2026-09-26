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

	// 精确定位会话：优先按 RemoteAddr（调用方携带 ConnSvrInfo.ClientPos），
	// 回退按 uid。随后按 Client.Transport 路由到拥有正确写路径的传输——
	// 旧实现固定走 WS，TCP/KCP 会话的服务端踢人（如心跳过期）不生效。
	client := globals.SessionHub.GetClientByRemoteAddr(req.GetRemoteAddr())
	if client == nil {
		client = globals.SessionHub.ClientForSend(ctx.Uid())
	}
	if client == nil || client.Conn == nil {
		ctx.Infof("conn kickout target not found {uid:%d, addr:%q}", ctx.Uid(), req.GetRemoteAddr())
		return nil, nil
	}

	switch client.Transport {
	case "tcp":
		globals.ConnTcpSvr.Kick(client.Uid, req.GetReason())
	case "kcp":
		globals.ConnKcpSvr.Kick(client.Uid, req.GetReason())
	default:
		// 兼容旧路径（含未打标记的存量会话）：按 remoteAddr 踢 WS。
		globals.ConnWsSvr.KickByRemoteAddr(client.Uid, req.GetReason(), client.RemoteAddr)
	}
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
