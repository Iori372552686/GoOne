package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStateTransitionsLegal(t *testing.T) {
	cases := []struct {
		from, to State
	}{
		{StateStarting, StateReady},
		{StateReady, StateAllocated},
		{StateAllocated, StateDraining},
		{StateDraining, StateStopping},
		{StateStopping, StateStopped},
		{StateStarting, StateFailed},
		{StateReady, StateFailed},
	}
	for _, c := range cases {
		if !canTransition(c.from, c.to) {
			t.Errorf("expected %s->%s to be legal", c.from, c.to)
		}
	}
}

func TestStateTransitionsIllegal(t *testing.T) {
	cases := []struct {
		from, to State
	}{
		{StateStopped, StateReady}, // terminal
		{StateFailed, StateReady},  // terminal
		{StateReady, StateStarting},
		{StateDraining, StateReady},
		{StateStopped, StateStopped},
	}
	for _, c := range cases {
		if canTransition(c.from, c.to) {
			t.Errorf("expected %s->%s to be illegal", c.from, c.to)
		}
	}
}

func TestStateStoreRejectsIllegalTransition(t *testing.T) {
	s := NewStateStore()
	// Force to Stopped, then try to go Ready (illegal).
	_ = s.transition(context.Background(), StateFailed, "boom", time.Time{})
	// Failed is terminal; any target is illegal.
	err := s.transition(context.Background(), StateReady, "nope", time.Time{})
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestReadyObserverFailureRollsBack(t *testing.T) {
	s := NewStateStore()
	s.AddObserver(ObserverFunc(func(_ context.Context, ch StateChange) error {
		if ch.Current == StateReady {
			return errors.New("not really ready")
		}
		return nil
	}))
	err := s.transition(context.Background(), StateReady, "started", time.Time{})
	if err == nil {
		t.Fatal("expected observer error to surface")
	}
	current, _ := s.Current()
	if current != StateStarting {
		t.Fatalf("expected rollback to Starting, got %s", current)
	}
}

func TestShutdownObserverFailureDoesNotBlock(t *testing.T) {
	s := NewStateStore()
	// Move to Ready first.
	if err := s.transition(context.Background(), StateReady, "ready", time.Time{}); err != nil {
		t.Fatalf("ready: %v", err)
	}
	s.AddObserver(ObserverFunc(func(_ context.Context, ch StateChange) error {
		if ch.Current == StateDraining {
			return errors.New("drain observer broke")
		}
		return nil
	}))
	// Draining transition must commit despite observer error.
	if err := s.transition(context.Background(), StateDraining, "sigterm", time.Time{}); err == nil {
		t.Fatal("expected observer error to be returned")
	}
	current, _ := s.Current()
	if current != StateDraining {
		t.Fatalf("expected Draining committed despite observer error, got %s", current)
	}
}

// TestRejectedReadyTransitionDoesNotFlipGauge 验证 P0-02 修复：当 Ready 观察者拒绝
// 转换时，状态回滚到前一状态，且 goone_lifecycle_state gauge 不得翻转。
//
// 历史缺陷：setLifecycleState(from,to) 曾在 transition 内部于"提交成功之前"调用，
// 因此被观察者拒绝的 Ready 转换会先把 starting->ready 的 gauge 翻转，再回滚状态，
// 导致 gauge 与实际状态不一致。
func TestRejectedReadyTransitionDoesNotFlipGauge(t *testing.T) {
	s := NewStateStore()
	s.AddObserver(ObserverFunc(func(_ context.Context, ch StateChange) error {
		if ch.Current == StateReady {
			return errors.New("not really ready")
		}
		return nil
	}))

	err := s.transition(context.Background(), StateReady, "started", time.Time{})
	if err == nil {
		t.Fatal("期望观察者拒绝 Ready 转换")
	}
	current, _ := s.Current()
	if current != StateStarting {
		t.Fatalf("期望回滚到 starting，got %s", current)
	}
	// gauge：starting=1，ready=0（拒绝不得翻转）。
	if got := testutil.ToFloat64(lifecycleStateGauge.WithLabelValues(string(StateStarting))); got != 1 {
		t.Fatalf("期望 starting gauge=1，got %v", got)
	}
	if got := testutil.ToFloat64(lifecycleStateGauge.WithLabelValues(string(StateReady))); got != 0 {
		t.Fatalf("拒绝的 Ready 转换不得翻转 gauge，ready gauge=%v", got)
	}
}

// TestNewStateStoreInitializesStartingGauge 验证：NewStateStore 在构造时把
// goone_lifecycle_state{state="starting"} 置为 1、其余状态置为 0，而不是等到第一次
// 转换才翻转。
func TestNewStateStoreInitializesStartingGauge(t *testing.T) {
	_ = NewStateStore()
	if got := testutil.ToFloat64(lifecycleStateGauge.WithLabelValues(string(StateStarting))); got != 1 {
		t.Fatalf("期望构造时 starting gauge=1，got %v", got)
	}
}

// TestAllocateSignatureReturnsErrorOnInvalidState 验证 P0-02：Allocate 接受
// context.Context 并返回 error，对非法状态返回 ErrInvalidStateTransition，对已
// Allocated 幂等返回 nil。
func TestAllocateSignatureReturnsErrorOnInvalidState(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runInBackground(t, a, ctx)
	waitForState(t, a, StateReady, 2*time.Second)

	// Ready -> Allocated：成功。
	if err := a.Allocate(context.Background()); err != nil {
		t.Fatalf("Ready->Allocated 期望 nil，got %v", err)
	}
	if a.State() != StateAllocated {
		t.Fatalf("期望 Allocated，got %s", a.State())
	}
	// Allocated -> Allocated：幂等 nil。
	if err := a.Allocate(context.Background()); err != nil {
		t.Fatalf("幂等 Allocate 期望 nil，got %v", err)
	}
	cancel()
	<-done

	// Stopped 后再 Allocate：ErrInvalidStateTransition。
	if err := a.Allocate(context.Background()); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("Stopped 后 Allocate 期望 ErrInvalidStateTransition，got %v", err)
	}
}

func TestAppReadyzFlipsImmediatelyOnDrain(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc", WithDrainTimeout(time.Minute))
	trace := make([]string, 0)
	// A component that blocks in Drain so we can observe the Draining state
	// before Stop runs.
	block := &blockingDrainer{name: "block", drainStarted: make(chan struct{})}
	mustRegister(a, block)
	mustRegister(a, newRecordingComponent("c", &trace, mu))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	// Wait until Ready.
	waitForState(t, a, StateReady, 2*time.Second)

	// Trigger drain and assert readyz is 503 as soon as Draining is entered,
	// before drain completes. Poll for the Draining state.
	cancel()
	waitForState(t, a, StateDraining, 2*time.Second)
	if code := readyCode(a.State()); code != 503 {
		t.Fatalf("readyz must be 503 in Draining, got %d (state=%s)", code, a.State())
	}
	if code := healthCode(a.State()); code != 200 {
		t.Fatalf("healthz must be 200 in Draining, got %d", code)
	}
	// Allow drain to proceed by cancelling via escalation.
	if e := a.tryEscalateDrain(); e != nil {
		e()
	}
	// 升级路径会返回 ErrDrainEscalated（非空），这是本测试主动触发的预期结果，
	// 不是失败。本测试只关心 readyz/healthz 的翻转，不校验错误语义。
	err := <-done
	if err != nil && !errors.Is(err, ErrDrainEscalated) {
		t.Fatalf("Run: %v", err)
	}
}

func TestAppReachesReadyAndStoppedStates(t *testing.T) {
	a, trace, mu := newTraceApp(t, "svc")
	mustRegister(a, newRecordingComponent("a", trace, mu))
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	waitForState(t, a, StateReady, 2*time.Second)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if a.State() != StateStopped {
		t.Fatalf("expected Stopped, got %s", a.State())
	}
}

func TestAllocateFlipsAllocatedFlag(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	waitForState(t, a, StateReady, 2*time.Second)
	if err := a.Allocate(context.Background()); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if a.State() != StateAllocated {
		t.Fatalf("expected Allocated, got %s", a.State())
	}
	// readyz still 200 when allocated.
	if code := readyCode(a.State()); code != 200 {
		t.Fatalf("readyz must be 200 when allocated, got %d", code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestAdminEndpointsServeStatez(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	admin := NewAdminComponent(a, WithAdminListen("127.0.0.1", 0))
	if err := admin.Start(context.Background()); err != nil {
		t.Fatalf("start admin: %v", err)
	}
	defer admin.Stop(context.Background())

	base := "http://" + admin.addr
	// /statez
	resp, body := getBody(t, base+"/statez")
	if resp != 200 {
		t.Fatalf("statez code=%d", resp)
	}
	if !strings.Contains(body, `"state"`) || !strings.Contains(body, `"service":"svc"`) {
		t.Fatalf("unexpected statez body: %s", body)
	}
	if strings.Contains(body, "password") || strings.Contains(body, "dsn") {
		t.Fatalf("statez must not leak credentials: %s", body)
	}
	// /components
	resp, body = getBody(t, base+"/components")
	if resp != 200 {
		t.Fatalf("components code=%d", resp)
	}
	if !strings.Contains(body, `"components"`) {
		t.Fatalf("unexpected components body: %s", body)
	}
}

func TestAdminDefaultsToLoopbackWhenIPEmpty(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	// Empty IP -> must bind 127.0.0.1, never 0.0.0.0.
	admin := NewAdminComponent(a, WithAdminListen("", 0))
	if err := admin.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer admin.Stop(context.Background())
	if !strings.HasPrefix(admin.addr, "127.0.0.1:") {
		t.Fatalf("admin must bind loopback when IP empty, got %s", admin.addr)
	}
}

// TestAdminReadyCheckFlipsReadyz 验证：注入的 readyCheck（如 router.ReadyCheck）在
// lifecycle Ready 时叠加判定；返回 error 则 readyz 返回 503（bus 断连摘流）。
func TestAdminReadyCheckFlipsReadyz(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc")
	trace := make([]string, 0)
	mustRegister(a, newRecordingComponent("c", &trace, mu))

	// readyCheck 先返回 nil（健康），后返回 error（故障）。
	healthy := true
	check := func() error {
		if !healthy {
			return errors.New("bus disconnected")
		}
		return nil
	}
	admin := NewAdminComponent(a,
		WithAdminListen("127.0.0.1", 0),
		WithAdminReadyCheck(check),
	)
	if err := admin.Start(context.Background()); err != nil {
		t.Fatalf("start admin: %v", err)
	}
	defer admin.Stop(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	waitForState(t, a, StateReady, 2*time.Second)

	base := "http://" + admin.addr
	// 健康：readyz 200。
	resp, _ := getBody(t, base+"/readyz")
	if resp != 200 {
		t.Fatalf("expected readyz 200 when healthy, got %d", resp)
	}
	// 故障：readyz 503。
	healthy = false
	resp, body := getBody(t, base+"/readyz")
	if resp != 503 {
		t.Fatalf("expected readyz 503 when check fails, got %d body=%s", resp, body)
	}
	// 恢复：readyz 200。
	healthy = true
	resp, _ = getBody(t, base+"/readyz")
	if resp != 200 {
		t.Fatalf("expected readyz 200 after recovery, got %d", resp)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestComponentTrackerRecordsTiming(t *testing.T) {
	tracker := NewComponentTracker([]string{"a", "b"})
	tracker.MarkStarting("a")
	tracker.MarkStarted("a", 5*time.Millisecond)
	tracker.MarkStartFailed("b", errors.New("boom"))
	report := tracker.Report()
	if len(report) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(report))
	}
	if report[0].Name != "a" || report[0].State != "running" || report[0].StartDurationMs < 0 {
		t.Fatalf("unexpected report[0]: %+v", report[0])
	}
	if report[1].Name != "b" || report[1].State != "failed" || report[1].LastError != "boom" {
		t.Fatalf("unexpected report[1]: %+v", report[1])
	}
}

// --- helpers ---

func waitForState(t *testing.T, a *App, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if a.State() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for state %s (current %s)", want, a.State())
}

func getBody(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return resp.StatusCode, string(buf)
}

// Ensure httptest import is used (the package is imported for completeness in
// future integration tests).
var _ = httptest.NewRecorder
