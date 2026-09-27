package ssrpc

import (
	"time"

	"github.com/golang/protobuf/proto"
)

const defaultSlowRequestThreshold = 50 * time.Millisecond

type LoggingOptions struct {
	SlowThreshold time.Duration
}

// Logging logs handler lifecycle with cmd/uid/rid/trans info (via ctx's logger methods).
//
// NOTE: this relies on cmd_handler.IContext's logging methods (Infof/Warningf/Errorf/Debugf).
func Logging() Middleware {
	return LoggingWithOptions(LoggingOptions{})
}

func LoggingWithOptions(opts LoggingOptions) Middleware {
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = defaultSlowRequestThreshold
	}
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if ctx == nil {
				return next(ctx, req)
			}
			start := time.Now()
			session := ctx.Session
			rsp, err := next(ctx, req)
			cost := time.Since(start)
			if session.TransID != 0 {
				if err != nil {
					ctx.Warningf("ssrpc err {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, trans:%v, reqType:%T, cost:%v} err=%v",
						session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, session.TransID, req, cost, err)
					return rsp, err // rsp 原样上抛：WrapUnary 错误回包契约依赖 rsp!=nil（吞 rsp=调用方超时）
				}
				if cost >= opts.SlowThreshold {
					ctx.Infof("ssrpc slow {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, trans:%v, reqType:%T, cost:%v, slowThreshold:%v}",
						session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, session.TransID, req, cost, opts.SlowThreshold)
					return rsp, nil
				}
				ctx.Debugf("ssrpc ok {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, trans:%v, reqType:%T, cost:%v}",
					session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, session.TransID, req, cost)
				return rsp, nil
			}
			if err != nil {
				ctx.Warningf("ssrpc err {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, reqType:%T, cost:%v} err=%v",
					session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, req, cost, err)
				return rsp, err // 同上：rsp 原样上抛，保住 WrapUnary 的错误回包
			}
			if cost >= opts.SlowThreshold {
				ctx.Infof("ssrpc slow {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, reqType:%T, cost:%v, slowThreshold:%v}",
					session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, req, cost, opts.SlowThreshold)
				return rsp, nil
			}
			ctx.Debugf("ssrpc ok {transport:%s, method:%s, cmd:%v, uid:%v, rid:%v, reqType:%T, cost:%v}",
				session.Transport, ctx.Method, uint32(ctx.Cmd), session.UID, session.RID, req, cost)
			return rsp, nil
		}
	}
}
