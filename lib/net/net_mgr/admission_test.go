package net_mgr

import (
	"testing"
)

// stubCounter 是测试用的 ActivityCounter，可控的连接/会话计数。
type stubCounter struct {
	conns int64
	sess  int64
}

func (s stubCounter) ActiveConnections() int64 { return s.conns }
func (s stubCounter) ActiveSessions() int64    { return s.sess }
func (s stubCounter) IncConnection()           {}
func (s stubCounter) DecConnection()           {}
func (s stubCounter) IncSession()              {}
func (s stubCounter) DecSession()              {}

// newTestHub 构造一个带 stubCounter 的 SessionHub，Accepting 为 true。
func newTestHub(conns, sess int64) *SessionHub {
	return NewSessionHub(stubCounter{conns: conns, sess: sess})
}

// TestAdmissionOffAlwaysAdmits 验证 off 模式直通。
func TestAdmissionOffAlwaysAdmits(t *testing.T) {
	hub := newTestHub(9999, 0)
	a := NewAdmissionController(hub, AdmissionLimits{
		MaxConnections: 10,
		OverloadMode:   OverloadModeOff,
	})
	for i := 0; i < 100; i++ {
		if !a.TryAdmitConnection() {
			t.Fatalf("off mode must always admit, rejected at %d", i)
		}
	}
}

// TestAdmissionShadowNeverRejects 验证 shadow 模式不拒绝（只记指标）。
func TestAdmissionShadowNeverRejects(t *testing.T) {
	hub := newTestHub(9999, 0)
	a := NewAdmissionController(hub, AdmissionLimits{
		MaxConnections: 1,
		OverloadMode:   OverloadModeShadow,
	})
	// 即使远超上限，shadow 也放行。
	for i := 0; i < 50; i++ {
		if !a.TryAdmitConnection() {
			t.Fatalf("shadow mode must not reject, rejected at %d", i)
		}
	}
}

// TestAdmissionEnforceRejectsOverMaxConnections 验证 enforce 超总连接上限拒绝。
func TestAdmissionEnforceRejectsOverMaxConnections(t *testing.T) {
	hub := newTestHub(100, 0) // 已达 100 连接
	a := NewAdmissionController(hub, AdmissionLimits{
		MaxConnections: 100,
		OverloadMode:   OverloadModeEnforce,
	})
	if a.TryAdmitConnection() {
		t.Fatal("enforce must reject when ActiveConnections >= MaxConnections")
	}
	// 低于上限则放行。
	hub2 := newTestHub(50, 0)
	a2 := NewAdmissionController(hub2, AdmissionLimits{MaxConnections: 100, OverloadMode: OverloadModeEnforce})
	if !a2.TryAdmitConnection() {
		t.Fatal("enforce must admit when below limit")
	}
}

// TestAdmissionEnforceRejectsOverUnauthenticated 验证 enforce 超未认证上限拒绝。
func TestAdmissionEnforceRejectsOverUnauthenticated(t *testing.T) {
	// 100 连接，10 已认证 → 90 未认证。
	hub := newTestHub(100, 10)
	a := NewAdmissionController(hub, AdmissionLimits{
		MaxUnauthenticatedConnections: 90,
		OverloadMode:                  OverloadModeEnforce,
	})
	if a.TryAdmitConnection() {
		t.Fatal("enforce must reject when unauthenticated >= limit")
	}
}

// TestAdmissionEnforceRejectsOnDraining 验证排空期（hub 不 Accepting）enforce 拒绝。
func TestAdmissionEnforceRejectsOnDraining(t *testing.T) {
	hub := newTestHub(0, 0)
	hub.Quiesce()
	a := NewAdmissionController(hub, AdmissionLimits{OverloadMode: OverloadModeEnforce})
	if a.TryAdmitConnection() {
		t.Fatal("enforce must reject during drain (hub not accepting)")
	}
}

// TestAdmissionLoginRateEnforce 验证 login_rate enforce 拒绝。
func TestAdmissionLoginRateEnforce(t *testing.T) {
	hub := newTestHub(0, 0)
	a := NewAdmissionController(hub, AdmissionLimits{
		LoginRate:    1,
		OverloadMode: OverloadModeEnforce,
	})
	// burst=1，第一次放行，后续速率耗尽时拒绝。
	if !a.TryAdmitLogin() {
		t.Fatal("first login should be admitted (burst)")
	}
	// 连续调用，至少有一次被拒（令牌桶容量有限）。
	rejected := false
	for i := 0; i < 100; i++ {
		if !a.TryAdmitLogin() {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Fatal("expected at least one login rejection under rate limit")
	}
}

// TestAdmissionInflightCounter 验证在途计数。
func TestAdmissionInflightCounter(t *testing.T) {
	a := NewAdmissionController(nil, AdmissionLimits{MaxInflight: 5, OverloadMode: OverloadModeEnforce})
	if a.Inflight() != 0 {
		t.Fatalf("initial inflight = %d, want 0", a.Inflight())
	}
	a.IncInflight()
	a.IncInflight()
	if a.Inflight() != 2 {
		t.Fatalf("inflight = %d, want 2", a.Inflight())
	}
	if a.InflightWouldReject(5) {
		t.Fatal("2 < 5 should not reject")
	}
	a.IncInflight()
	a.IncInflight()
	a.IncInflight() // now 5
	if !a.InflightWouldReject(5) {
		t.Fatal("5 >= 5 should reject")
	}
	a.DecInflight()
	if a.Inflight() != 4 {
		t.Fatalf("inflight = %d, want 4", a.Inflight())
	}
}

// TestNilAdmissionControllerSafe 验证 nil controller 安全（直通）。
func TestNilAdmissionControllerSafe(t *testing.T) {
	var a *AdmissionController
	if !a.TryAdmitConnection() {
		t.Fatal("nil controller must admit (safe default)")
	}
	if !a.TryAdmitLogin() {
		t.Fatal("nil controller must admit login")
	}
}
