package ssrpc

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/golang/protobuf/proto"
)

// ErrOverloaded 是 SSRPC 过载拒绝的哨兵错误。
// admission middleware 在 enforce 模式超 max_inflight 时返回此错误，调用方可通过
// errors.Is 判断是否为过载拒绝。
var ErrOverloaded = ssrpcError("ssrpc: request rejected by admission control (overloaded)")

// ssrpcError 是一个实现 error 接口的简单字符串错误类型，支持 errors.Is 比较。
type ssrpcError string

func (e ssrpcError) Error() string { return string(e) }

// InflightLimiter 原子地占用与释放 SSRPC 在途请求名额。
// net_mgr.AdmissionController 实现此接口，使 ssrpc 中间件无需 import net_mgr
// （避免循环依赖）。
//
// TryAcquireInflight 用原子 CAS 完成"检查上限 + 占位"，避免历史 check-then-act 超限。
// 成功后调用方必须配对调用 ReleaseInflight(method)。
type InflightLimiter interface {
	// TryAcquireInflight 原子占用一个名额。globalLimit/methodLimit 任一 > 0 启用，都 <= 0
	// 表示不限。返回 true 表示占用成功。shadow 模式不拒绝（返回 true）但计数仍正确。
	TryAcquireInflight(method string, globalLimit, methodLimit int) bool
	// ReleaseInflight 释放一个已占用名额，必须与成功的 Acquire 配对。
	ReleaseInflight(method string)
}

// rejectLogMu/rejectLogLastSeen 实现拒绝日志的按 reason 限频采样。
// 过载时不再逐请求 Warning（会放大 I/O），改为每个 reason 至多每秒记录一次首个样本。
var (
	rejectLogMu      sync.Mutex
	rejectLogLastSec = make(map[string]int64)
)

// logRejectSample 按 reason 限频记录一次拒绝日志：每个 method 至多每秒一条。
func logRejectSample(method string) {
	now := time.Now().Unix()
	rejectLogMu.Lock()
	last := rejectLogLastSec[method]
	if now-last >= 1 {
		rejectLogLastSec[method] = now
		rejectLogMu.Unlock()
		logger.Warningf("ssrpc inflight overloaded, reject sample method=%s (rate-limited)", method)
		return
	}
	rejectLogMu.Unlock()
}

// AdmissionMiddleware 构造一个 SSRPC 在途请求限流中间件（原子化）。
//
// limiter 为 nil 时直通（无 admission）。每个请求用 TryAcquireInflight 原子占位，处理完
// 用 ReleaseInflight 释放；占位失败（enforce 满载）返回 ErrOverloaded，不调用下游 handler。
//
// 注意：shadow/enforce 的策略由 limiter 实现侧（AdmissionController）决定；本中间件只做
// 原子占位/释放与拒绝。拒绝日志按 reason 限频采样，过载 60s 日志量保持有界。
func AdmissionMiddleware(limiter InflightLimiter, maxInflight int, maxPerMethod map[string]int) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if limiter == nil {
				return next(ctx, req)
			}
			// 选定本方法的 inflight 上限：per-method 覆盖优先，否则全局。
			methodLimit := 0
			if m, ok := maxPerMethod[ctx.Method]; ok && m > 0 {
				methodLimit = m
			}
			if globalLimit := maxInflight; globalLimit > 0 || methodLimit > 0 {
				if !limiter.TryAcquireInflight(ctx.Method, globalLimit, methodLimit) {
					// 过载（enforce）：不进入下游，不占名额（Acquire 已回滚）。
					atomic.AddInt64(&admissionRejectCount, 1)
					logRejectSample(ctx.Method)
					return nil, ErrOverloaded
				}
				defer limiter.ReleaseInflight(ctx.Method)
			}
			return next(ctx, req)
		}
	}
}

// admissionRejectCount 累计过载拒绝次数，供指标/测试观测。
var admissionRejectCount int64

// AdmissionRejectCount 返回累计过载拒绝次数（测试与指标用）。
func AdmissionRejectCount() int64 { return atomic.LoadInt64(&admissionRejectCount) }
