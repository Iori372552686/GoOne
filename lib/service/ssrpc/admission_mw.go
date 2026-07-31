package ssrpc

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/golang/protobuf/proto"
)

// ErrOverloaded 是 SSRPC 过载拒绝的哨兵错误（V3-P1-01）。
// admission middleware 在 enforce 模式超 max_inflight 时返回此错误，调用方可通过
// errors.Is 判断是否为过载拒绝。
var ErrOverloaded = ssrpcError("ssrpc: request rejected by admission control (overloaded)")

// ssrpcError 是一个实现 error 接口的简单字符串错误类型，支持 errors.Is 比较。
type ssrpcError string

func (e ssrpcError) Error() string { return string(e) }

// InflightLimiter 上报当前在途计数与是否应拒绝（V3-P1-01）。
// net_mgr.AdmissionController 实现此接口，使 ssrpc 中间件无需 import net_mgr
//（避免循环依赖）。
type InflightLimiter interface {
	IncInflight() int64
	DecInflight() int64
	// InflightWouldReject 上报当前在途是否达到 max（按全局或 per-method）。
	// method 为 RPC 方法名（ctx.Method）；max<=0 表示不限。
	InflightWouldReject(max int) bool
}

// AdmissionMiddleware 构造一个 SSRPC 在途请求限流中间件（V3-P1-01）。
//
// limiter 为 nil 时直通（无 admission）。limiter 非 nil 但 maxInflight<=0 时也直通。
// 每个请求 IncInflight，处理完 DecInflight；进入前若 InflightWouldReject 则返回
// ErrOverloaded（不调用下游 handler）。
//
// 注意：本中间件只做计数与拒绝决策；shadow/enforce 模式的策略由 limiter 实现侧
//（AdmissionController）决定。InflightWouldReject 在 enforce 满载时返回 true。
func AdmissionMiddleware(limiter InflightLimiter, maxInflight int, maxPerMethod map[string]int) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if limiter == nil {
				return next(ctx, req)
			}
			// 选定本方法的 inflight 上限：per-method 覆盖优先，否则全局。
			max := maxInflight
			if m, ok := maxPerMethod[ctx.Method]; ok && m > 0 {
				max = m
			}
			if max > 0 && limiter.InflightWouldReject(max) {
				// 过载：不进入下游，不增减计数（已达上限）。
				logger.Warningf("ssrpc inflight overloaded, reject method=%s", ctx.Method)
				return nil, ErrOverloaded
			}
			limiter.IncInflight()
			defer limiter.DecInflight()
			return next(ctx, req)
		}
	}
}
