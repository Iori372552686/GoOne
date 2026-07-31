package net_mgr

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// OverloadMode 控制过载保护策略（V3-P1-01）。
const (
	OverloadModeOff     = "off"     // 不限制（向后兼容默认）。
	OverloadModeShadow  = "shadow"  // 只统计上报，不拒绝。
	OverloadModeEnforce = "enforce" // 超限拒绝。
)

// AdmissionController 是网关连接与登录的过载保护闸门（V3-P1-01）。
//
// 它持有容量上限与令牌桶限速器，基于 SessionHub 的连接/会话计数做决策。
// OverloadMode=off 时所有检查直通；shadow 时只记录指标不拒绝；enforce 时超限拒绝。
//
// Quiesce 语义协同：SessionHub 不再 Accepting 时（排空期），连接 admission 强制拒绝，
// 等价于 max_connections=0，保证排空优先于普通 admission 判断。
//
// 原子租约（V4 P0-03 修复）：连接与 inflight 准入改为原子 Acquire/Release，避免历史
// "检查后增加"在并发下超过上限。连接名额由 reservedConns 原子维护；inflight 名额由
// 全局 inflight + 有界 per-method 计数器维护。shadow 模式执行相同决策计算但不拒绝，
// 计数仍正确 Acquire/Release 以保证 shadow/enforce 可对账。
//
// 并发安全：rate.Limiter 自身线程安全；limits 在装配期设置后只读；所有计数用 atomic
// 或 per-method 锁保护。
type AdmissionController struct {
	hub    *SessionHub
	limits AdmissionLimits

	// connLimiter 限制每秒新建连接数；loginLimiter 限制每秒首次登录数。
	// rate.Limit(0) 表示无限，与 limits 为 0（不限）一致。
	connLimiter  *rate.Limiter
	loginLimiter *rate.Limiter

	// reservedConns 是已占用的连接名额（原子 Acquire/Release）。TryAcquireConnection 用
	// CAS 循环完成"检查上限 + 占位"，避免 check-then-act 超限。
	reservedConns int64

	// inflight 为 SSRPC 在途请求的全局计数（供 ssrpc admission middleware 复用）。
	inflight int64

	// methodInflight 按 method 维护在途计数；methodMu 保护 map，每个计数用 *int64 原子
	// 操作。只为配置中出现的方法创建计数器，不允许按任意客户端输入无限增长 map。
	methodMu       sync.Mutex
	methodInflight map[string]*int64
}

// AdmissionLimits 是 AdmissionController 的不可变配置快照。
type AdmissionLimits struct {
	MaxConnections                int64
	MaxUnauthenticatedConnections int64
	ConnectionRate                int // 每秒，0=不限
	LoginRate                     int // 每秒，0=不限
	MaxInflight                   int
	OverloadMode                  string
}

// NewAdmissionController 构造一个闸门。hub 用于查询实时连接/会话计数与排空状态。
// limits 为零值时等价于 OverloadModeOff（不限）。
func NewAdmissionController(hub *SessionHub, limits AdmissionLimits) *AdmissionController {
	return &AdmissionController{
		hub:            hub,
		limits:         limits,
		connLimiter:    rate.NewLimiter(rateLimit(limits.ConnectionRate), burst(limits.ConnectionRate)),
		loginLimiter:   rate.NewLimiter(rateLimit(limits.LoginRate), burst(limits.LoginRate)),
		methodInflight: make(map[string]*int64),
	}
}

// rateLimit 把"每秒 N 次"转为 rate.Limit；0 表示无限。
func rateLimit(perSec int) rate.Limit {
	if perSec <= 0 {
		return rate.Inf
	}
	return rate.Limit(perSec)
}

// burst 取令牌桶容量。不限时给一个大 burst，避免限速器意外节流。
func burst(perSec int) int {
	if perSec <= 0 {
		return 1 << 30
	}
	return perSec
}

// enabled 上报是否启用了 admission（off 或无 limits 时为 false）。
func (a *AdmissionController) enabled() bool {
	if a == nil {
		return false
	}
	switch a.limits.OverloadMode {
	case OverloadModeShadow, OverloadModeEnforce:
		return true
	default:
		return false
	}
}

// TryAdmitConnection 在接受一个新连接前调用。返回 true 表示放行，false 表示拒绝。
// shadow 模式总是返回 true（但记录 shadow_reject 指标）；enforce 模式超限返回 false。
// 排空期（hub 不 Accepting）在 enforce 模式下强制拒绝。
func (a *AdmissionController) TryAdmitConnection() bool {
	if !a.enabled() {
		return true
	}
	// 排空期协同：Quiesce 后拒绝新连接。
	if a.hub != nil && !a.hub.Accepting() {
		recordAdmission("connection", decisionFor(a, false), "draining")
		return a.admitOrReject(false)
	}

	reason := ""
	admit := true
	// 总连接数上限。
	if a.limits.MaxConnections > 0 && a.hub != nil {
		if a.hub.ActiveConnections() >= a.limits.MaxConnections {
			admit, reason = false, "max_connections"
		}
	}
	// 未认证连接上限 = 总连接 - 已认证会话。
	if admit && a.limits.MaxUnauthenticatedConnections > 0 && a.hub != nil {
		unauth := a.hub.ActiveConnections() - a.hub.ActiveSessions()
		if unauth >= a.limits.MaxUnauthenticatedConnections {
			admit, reason = false, "max_unauthenticated"
		}
	}
	// 连接速率（令牌桶）。
	if admit && !a.connLimiter.Allow() {
		admit, reason = false, "connection_rate"
	}

	recordAdmission("connection", decisionFor(a, admit), reason)
	return a.admitOrReject(admit)
}

// TryAdmitLogin 在首次登录（BindClient）前调用。返回 true 表示放行。
// 仅 enforce 模式下超限返回 false。
func (a *AdmissionController) TryAdmitLogin() bool {
	if !a.enabled() {
		return true
	}
	if a.hub != nil && !a.hub.Accepting() {
		recordAdmission("login", decisionFor(a, false), "draining")
		return a.admitOrReject(false)
	}
	admit := a.loginLimiter.Allow()
	reason := ""
	if !admit {
		reason = "login_rate"
	}
	recordAdmission("login", decisionFor(a, admit), reason)
	return a.admitOrReject(admit)
}

// TryAcquireConnection 原子地占用一个连接名额（V4 P0-03）。返回 true 表示成功占用，
// 调用方在连接释放时必须配对调用 ReleaseConnection。
//
// 用 CAS 循环完成"检查上限 + 占位"，避免历史 check-then-act 在并发下超过上限。
// 排空期（hub 不 Accepting）强制拒绝；连接速率（令牌桶）在占位前检查。
// shadow 模式执行相同决策计算但不占用名额（仍返回 true 以放行）。
func (a *AdmissionController) TryAcquireConnection() bool {
	if !a.enabled() {
		return true
	}
	// 排空期协同：Quiesce 后拒绝新连接。
	if a.hub != nil && !a.hub.Accepting() {
		recordAdmission("connection", decisionFor(a, false), "draining")
		return a.admitOrReject(false)
	}
	// 连接速率（令牌桶）：rate.Limiter 自身线程安全，先消费令牌再占位。
	if !a.connLimiter.Allow() {
		recordAdmission("connection", decisionFor(a, false), "connection_rate")
		return a.admitOrReject(false)
	}

	maxConn := a.limits.MaxConnections
	if maxConn <= 0 {
		// 无总连接上限：shadow/enforce 都放行。仍记录 admit 指标。
		recordAdmission("connection", decisionFor(a, true), "")
		return true
	}

	// CAS 占位：只有当前占用 < 上限时才 +1。
	for {
		cur := atomic.LoadInt64(&a.reservedConns)
		if cur >= maxConn {
			recordAdmission("connection", decisionFor(a, false), "max_connections")
			return a.admitOrReject(false)
		}
		if atomic.CompareAndSwapInt64(&a.reservedConns, cur, cur+1) {
			recordAdmission("connection", decisionFor(a, true), "")
			return true
		}
		// CAS 失败：他人抢先占位，重试。
	}
}

// ReleaseConnection 释放一个已占用的连接名额。仅释放已获得的租约，绝不降到负数。
func (a *AdmissionController) ReleaseConnection() {
	if a == nil {
		return
	}
	for {
		cur := atomic.LoadInt64(&a.reservedConns)
		if cur <= 0 {
			// 已为 0：不向下减，避免负数（防止重复 Release 或未 Acquire 的 Release）。
			return
		}
		if atomic.CompareAndSwapInt64(&a.reservedConns, cur, cur-1) {
			return
		}
	}
}

// ReservedConnections 返回当前已占用的连接名额（测试与指标观测用）。
func (a *AdmissionController) ReservedConnections() int64 {
	if a == nil {
		return 0
	}
	return atomic.LoadInt64(&a.reservedConns)
}

// admitOrReject 按 OverloadMode 把"是否超限"映射为实际决策。
// shadow 模式：超限也放行（但指标已记为 shadow_reject）。
// enforce 模式：超限则拒绝。
func (a *AdmissionController) admitOrReject(within bool) bool {
	if within {
		return true
	}
	return a.limits.OverloadMode != OverloadModeEnforce
}

// decisionFor 返回指标用的 decision 值。
func decisionFor(a *AdmissionController, within bool) string {
	if within {
		return "admit"
	}
	if a.limits.OverloadMode == OverloadModeShadow {
		return "shadow_reject"
	}
	return "reject"
}

// IncInflight / DecInflight 维护 SSRPC 在途请求计数（供 ssrpc admission 复用）。
// Deprecated: 保留一个版本供旧调用方；新代码应使用 TryAcquireInflight/ReleaseInflight
// 以获得原子检查与计数（V4 P0-03）。
func (a *AdmissionController) IncInflight() int64 { return atomic.AddInt64(&a.inflight, 1) }
func (a *AdmissionController) DecInflight() int64 { return atomic.AddInt64(&a.inflight, -1) }
func (a *AdmissionController) Inflight() int64    { return atomic.LoadInt64(&a.inflight) }

// InflightWouldReject 上报当前全局在途计数是否达到 max。max<=0 表示不限。
// Deprecated: 新代码应使用 TryAcquireInflight（原子检查+占位），避免 check-then-act 超限。
func (a *AdmissionController) InflightWouldReject(max int) bool {
	if max <= 0 {
		return false
	}
	return a.Inflight() >= int64(max)
}

// TryAcquireInflight 原子地占用一个在途请求名额（V4 P0-03）。
//
// 同时检查全局上限 globalLimit 与本方法上限 methodLimit（任一 > 0 时启用）；两者都 <= 0
// 表示不限。用 CAS 循环完成"检查 + 占位"，避免并发超限。成功后调用方必须配对调用
// ReleaseInflight(method)。method 计数器只为 methodLimit>0 的方法懒创建，不允许按任意
// 客户端输入无限增长 map。
//
// shadow 模式：执行相同决策计算与指标记录，但不拒绝（返回 true）。计数仍正确增减，使
// shadow/enforce 可对账。
func (a *AdmissionController) TryAcquireInflight(method string, globalLimit, methodLimit int) bool {
	if a == nil {
		return true
	}
	// 都不限：放行，不占用全局计数（保留向后兼容语义——旧路径 Inc/Dec 由中间件负责）。
	if globalLimit <= 0 && methodLimit <= 0 {
		return true
	}

	// 全局 CAS 占位。
	if globalLimit > 0 {
		for {
			cur := atomic.LoadInt64(&a.inflight)
			if cur >= int64(globalLimit) {
				return a.admitOrReject(false)
			}
			if atomic.CompareAndSwapInt64(&a.inflight, cur, cur+1) {
				break
			}
		}
	} else {
		// 无全局上限也要占用以便 per-method 释放平衡；但用单独的非 CAS 增量。
		atomic.AddInt64(&a.inflight, 1)
	}

	// per-method 占位（仅当该方法有上限）。
	if methodLimit > 0 {
		mc := a.methodCounter(method)
		for {
			cur := atomic.LoadInt64(mc)
			if cur >= int64(methodLimit) {
				// 方法满：回滚刚才的全局占位。
				atomic.AddInt64(&a.inflight, -1)
				return a.admitOrReject(false)
			}
			if atomic.CompareAndSwapInt64(mc, cur, cur+1) {
				return true
			}
		}
	}
	return true
}

// ReleaseInflight 释放一个在途请求名额。必须与成功的 TryAcquireInflight(method) 配对。
func (a *AdmissionController) ReleaseInflight(method string) {
	if a == nil {
		return
	}
	// 释放 per-method 与全局（两者都减，保证占用平衡）。
	if mc := a.peekMethodCounter(method); mc != nil {
		for {
			cur := atomic.LoadInt64(mc)
			if cur <= 0 {
				break
			}
			if atomic.CompareAndSwapInt64(mc, cur, cur-1) {
				break
			}
		}
	}
	for {
		cur := atomic.LoadInt64(&a.inflight)
		if cur <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&a.inflight, cur, cur-1) {
			return
		}
	}
}

// MethodInflight 返回指定方法的当前在途计数（测试与指标观测用）。
func (a *AdmissionController) MethodInflight(method string) int64 {
	if a == nil {
		return 0
	}
	if mc := a.peekMethodCounter(method); mc != nil {
		return atomic.LoadInt64(mc)
	}
	return 0
}

// methodCounter 懒创建并返回指定方法的计数器指针。
func (a *AdmissionController) methodCounter(method string) *int64 {
	a.methodMu.Lock()
	defer a.methodMu.Unlock()
	mc, ok := a.methodInflight[method]
	if !ok {
		var v int64
		mc = &v
		a.methodInflight[method] = mc
	}
	return mc
}

// peekMethodCounter 返回已存在的计数器；不存在返回 nil（不加锁创建）。
func (a *AdmissionController) peekMethodCounter(method string) *int64 {
	a.methodMu.Lock()
	defer a.methodMu.Unlock()
	return a.methodInflight[method]
}

// AdmitInterval 仅用于测试：返回闸门的令牌桶恢复等待参考，生产不调用。
func (a *AdmissionController) AdmitInterval() time.Duration { return time.Second }
