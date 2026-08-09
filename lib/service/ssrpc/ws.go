package ssrpc

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

type transportHintIContext interface {
	SSRPCTransport() Transport
}

func transportForClientPacket(ic cmd_handler.IContext) Transport {
	if hinted, ok := any(ic).(transportHintIContext); ok {
		if transport := hinted.SSRPCTransport(); transport != "" {
			return transport
		}
	}
	return TransportWS
}

// WrapWS returns a CmdHandlerFunc for the WS (CSPacket) transport.
//
// Structurally identical to WrapUnary but stamps a client-packet transport on
// the Context. It defaults to TransportWS and allows an IContext hint to
// override that (e.g. connsvr raw TCP clients).
func WrapWS(desc MethodDesc, mws []Middleware, newReq func() any, invoke func(ctx *Context, req any) (any, error)) cmd_handler.CmdHandlerFunc {
	mws = prepareMW(mws, desc.UIDLock)
	h := buildHandler(mws, invoke) // pre-build chain once at init time
	return func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		if c == nil {
			return g1_protocol.ErrorCode_ERR_INTERNAL
		}
		ctx := WrapIContext(c, desc.Cmd)
		ctx.SetTransport(transportForClientPacket(c))
		applyDesc(ctx, &desc)
		ctx.ApplyTimeout(effectiveMethodTimeout(desc.Timeout))
		defer ctx.Close()

		reqAny := newReq()
		req, ok := reqAny.(proto.Message)
		if !ok || req == nil {
			ctx.Warningf("ssrpc.ws invalid req type: %T", reqAny)
			return g1_protocol.ErrorCode_ERR_INTERNAL
		}
		if err := ctx.ParseMsg(data, req); err != nil {
			ctx.Warningf("ssrpc.ws parse failed err=%v", err)
			return g1_protocol.ErrorCode_ERR_MARSHAL
		}

		rsp, err := h(ctx, req)
		if err != nil {
			return ToErrorCode(err)
		}
		if desc.OneWay {
			return g1_protocol.ErrorCode_ERR_OK
		}
		if rsp != nil {
			cmdResp := desc.CmdResp
			if cmdResp == 0 {
				cmdResp = g1_protocol.CMD(uint32(desc.Cmd) + 1)
			}
			SendMsgBackWithCmd(ctx, cmdResp, rsp)
		}
		return g1_protocol.ErrorCode_ERR_OK
	}
}
