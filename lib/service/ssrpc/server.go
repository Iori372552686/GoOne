package ssrpc

import (
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

const DefaultMethodTimeout = 5 * time.Second

// MethodDesc describes one RPC method exposed via any transport.
//
// This is primarily used by code generation to keep wrapper logic consistent.
type MethodDesc struct {
	Cmd       g1_protocol.CMD
	CmdResp   g1_protocol.CMD // 0 means default Cmd+1
	OneWay    bool
	UIDLock   bool
	Auth      bool
	Sign      bool
	Timeout   time.Duration
	TraceTags map[string]string
	Name      string
}

// ---------------------------------------------------------------------------
// Shared internal helpers -- eliminate duplication across Wrap* functions.
// ---------------------------------------------------------------------------

// prepareMW clones the middleware slice and appends UIDLock if needed.
// This MUST be called at init time (not per-request) to avoid slice mutation.
func prepareMW(mws []Middleware, uidLock bool) []Middleware {
	if !uidLock {
		return mws
	}
	out := make([]Middleware, len(mws), len(mws)+1)
	copy(out, mws)
	return append(out, UIDLock())
}

// applyDesc stamps MethodDesc metadata onto a Context.
func applyDesc(ctx *Context, desc *MethodDesc) {
	ctx.SetCmd(desc.Cmd)
	ctx.SetMethod(desc.Name)
	ctx.AuthRequired = desc.Auth
	ctx.SignRequired = desc.Sign
	if desc.TraceTags != nil {
		m := make(map[string]string, len(desc.TraceTags))
		for k, v := range desc.TraceTags {
			m[k] = v
		}
		ctx.TraceTags = m
	} else {
		ctx.TraceTags = nil
	}
}

func effectiveMethodTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return DefaultMethodTimeout
}

// buildHandler wraps an invoke function into a Handler and chains middleware.
func buildHandler(mws []Middleware, invoke func(ctx *Context, req any) (any, error)) Handler {
	h := Handler(func(ctx *Context, in proto.Message) (proto.Message, error) {
		outAny, err := invoke(ctx, in)
		if outAny == nil {
			return nil, err
		}
		out, ok := outAny.(proto.Message)
		if !ok {
			return nil, Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "ssrpc invalid rsp type", nil)
		}
		return out, err
	})
	if len(mws) > 0 {
		h = Chain(mws...)(h)
	}
	return h
}

// ---------------------------------------------------------------------------
// WrapUnary -- SSPacket transport
// ---------------------------------------------------------------------------

// WrapUnary returns a TransactionMgr-compatible command handler.
//
// It wraps IContext -> Context, unmarshals via ParseMsg, runs middleware,
// maps errors via ToErrorCode, and auto-replies unless OneWay.
//
// NOTE: newReq/invoke use `any` so generated code does NOT need to import proto.
func WrapUnary(desc MethodDesc, mws []Middleware, newReq func() any, invoke func(ctx *Context, req any) (any, error)) cmd_handler.CmdHandlerFunc {
	mws = prepareMW(mws, desc.UIDLock)
	h := buildHandler(mws, invoke) // pre-build chain once at init time
	return func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		if c == nil {
			return g1_protocol.ErrorCode_ERR_INTERNAL
		}
		ctx := WrapIContext(c, desc.Cmd)
		ctx.SetTransport(TransportSS)
		applyDesc(ctx, &desc)
		ctx.ApplyTimeout(effectiveMethodTimeout(desc.Timeout))
		defer ctx.Close()

		reqAny := newReq()
		req, ok := reqAny.(proto.Message)
		if !ok || req == nil {
			ctx.Warningf("ssrpc invalid req type: %T", reqAny)
			return g1_protocol.ErrorCode_ERR_INTERNAL
		}
		if err := ctx.ParseMsg(data, req); err != nil {
			ctx.Warningf("ssrpc parse failed err=%v", err)
			return g1_protocol.ErrorCode_ERR_MARSHAL
		}

		rsp, err := h(ctx, req)
		if err != nil {
			code := ToErrorCode(err)
			// 业务 handler 返回 (rsp, err) 时：把 err 的错误码写进 rsp 并回包，
			// 使业务可统一 return rsp, gerr.New(...) —— 框架自动设码 + 回包。
			// rsp==nil（系统级错误）时不回包，只返回码给框架记账。
			if !desc.OneWay && rsp != nil {
				ApplyErrCode(rsp, code)
				cmdResp := desc.CmdResp
				if cmdResp == 0 {
					cmdResp = g1_protocol.CMD(uint32(desc.Cmd) + 1)
				}
				SendMsgBackWithCmd(ctx, cmdResp, rsp)
			}
			return code
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
