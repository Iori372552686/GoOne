package net_mgr

import (
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
// 并发安全：rate.Limiter 自身线程安全；limits 在装配期设置后只读。
type AdmissionController struct {
	hub    *SessionHub
	limits AdmissionLimits

	// connLimiter 限制每秒新建连接数；loginLimiter 限制每秒首次登录数。
	// rate.Limit(0) 表示无限，与 limits 为 0（不限）一致。
	connLimiter  *rate.Limiter
	loginLimiter *rate.Limiter

	// inflight 为 SSRPC 在途请求计数（供 ssrpc admission middleware 复用）。
	// 这里仅维护计数，拒绝逻辑在 ssrpc 层。
	inflight int64
}

// AdmissionLimits 是 AdmissionController 的不可变配置快照。
type AdmissionLimits struct {
	MaxConnections               int64
	MaxUnauthenticatedConnections int64
	ConnectionRate               int // 每秒，0=不限
	LoginRate                    int // 每秒，0=不限
	MaxInflight                  int
	OverloadMode                 string
}

// NewAdmissionController 构造一个闸门。hub 用于查询实时连接/会话计数与排空状态。
// limits 为零值时等价于 OverloadModeOff（不限）。
func NewAdmissionController(hub *SessionHub, limits AdmissionLimits) *AdmissionController {
	return &AdmissionController{
		hub:          hub,
		limits:       limits,
		connLimiter:  rate.NewLimiter(rateLimit(limits.ConnectionRate), burst(limits.ConnectionRate)),
		loginLimiter: rate.NewLimiter(rateLimit(limits.LoginRate), burst(limits.LoginRate)),
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
func (a *AdmissionController) IncInflight() int64 { return atomic.AddInt64(&a.inflight, 1) }
func (a *AdmissionController) DecInflight() int64 { return atomic.AddInt64(&a.inflight, -1) }
func (a *AdmissionController) Inflight() int64    { return atomic.LoadInt64(&a.inflight) }

// InflightWouldReject 上报当前在途计数是否超过 max。max<=0 表示不限。
func (a *AdmissionController) InflightWouldReject(max int) bool {
	if max <= 0 {
		return false
	}
	return a.Inflight() >= int64(max)
}

// AdmitInterval 仅用于测试：返回闸门的令牌桶恢复等待参考，生产不调用。
func (a *AdmissionController) AdmitInterval() time.Duration { return time.Second }
