package ssrpc

// DefaultMWOptions configures the default middleware chain for ssrpc servers.
//
// The defaults are intentionally conservative:
// - Recover/Logging are always included
// - Trace is included (currently a no-op placeholder)
// - Metrics is included (built-in Prometheus recorder when Metrics=nil)
// - MCP is attached/guarded only when MCP is non-nil
// - Extra middlewares are appended at the end
type DefaultMWOptions struct {
	Trace     TraceProvider
	Auth      Authenticator
	Sign      SignVerifier
	UIDLocker UIDLocker
	Metrics   MetricsRecorder
	MCP       MCP
	MCPGuard  MCPGuardFunc
	Extra     []Middleware

	// InflightLimiter 控制 SSRPC 在途请求过载保护（V3-P1-01）。nil 时无 inflight 限流。
	InflightLimiter InflightLimiter
	// MaxInflight 全局在途上限；MaxInflightPerMethod 按 method 名覆盖。0=不限。
	MaxInflight          int
	MaxInflightPerMethod map[string]int
}

// DefaultMiddlewares returns a standard middleware chain for SSPacket RPC.
func DefaultMiddlewares(opts DefaultMWOptions) []Middleware {
	recorder := opts.Metrics
	if recorder == nil {
		recorder = DefaultMetricsRecorder()
	}
	mws := []Middleware{
		Recover(),
		Logging(),
	}
	// V3-P1-01：inflight admission 紧跟 Logging 之后、Auth/UIDLock 之前，
	// 使过载拒绝不占用认证/加锁资源。
	if opts.InflightLimiter != nil && opts.MaxInflight > 0 {
		mws = append(mws, AdmissionMiddleware(opts.InflightLimiter, opts.MaxInflight, opts.MaxInflightPerMethod))
	}
	mws = append(mws,
		TraceWith(opts.Trace),
		AuthWith(opts.Auth),
		SignWith(opts.Sign),
		UIDLockAttach(opts.UIDLocker),
		Metrics(recorder),
	)
	if opts.MCP != nil {
		mws = append(mws, MCPAttach(opts.MCP), MCPGuardWith(opts.MCPGuard))
	}
	if len(opts.Extra) > 0 {
		mws = append(mws, opts.Extra...)
	}
	return mws
}
