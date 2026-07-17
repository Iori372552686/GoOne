package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestIntegration_FullLifecycleWithDrain 验证完整生命周期闭环：
// Ready → Draining → Stopping → Stopped，且组件按注册顺序 Start、逆序 Stop。
func TestIntegration_FullLifecycleWithDrain(t *testing.T) {
	var mu sync.Mutex
	trace := make([]string, 0)
	mk := func(name string, withDrain bool) Component {
		return &integrationComp{name: name, mu: &mu, trace: &trace, hasDrain: withDrain}
	}
	a, err := New("itest",
		WithDrainTimeout(time.Second),
		WithStopTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, c := range []Component{mk("a", true), mk("b", true), mk("c", false)} {
		if err := a.Register(c); err != nil {
			t.Fatalf("register %s: %v", c.Name(), err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), trace...)
	mu.Unlock()

	// Start 顺序 a,b,c。
	assertSubsequence(t, got, []string{"a:start", "b:start", "c:start"}, "start order")
	// Drain 仅对实现了 Drainer 的 a,b（逆序 b,a）。
	assertSubsequence(t, got, []string{"b:drain", "a:drain"}, "drain reverse order")
	// Stop 全部逆序 c,b,a。
	assertSubsequence(t, got, []string{"c:stop", "b:stop", "a:stop"}, "stop reverse order")

	if a.State() != StateStopped {
		t.Fatalf("expected Stopped, got %s", a.State())
	}
}

// TestIntegration_DrainTimeoutForcesStop 验证：Drain 永不完成时，drain_timeout 到期
// 后仍进入 Stop 并返回（进程不挂死）。
func TestIntegration_DrainTimeoutForcesStop(t *testing.T) {
	a, err := New("itest", WithDrainTimeout(50*time.Millisecond), WithStopTimeout(time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	if err := a.Register(slow); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	start := time.Now()
	if err := <-done; err != nil {
		// drain ctx 超时返回的 error 可接受；关键是 Run 返回。
		_ = err
	}
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Fatalf("drain timeout did not force stop; elapsed=%v", elapsed)
	}
	if !slow.stopped.Load() {
		t.Fatal("expected Stop after drain timeout")
	}
}

// TestIntegration_SecondSignalEscalatesDrain 验证：第二信号（escalation）取消进行中的
// Drain，立即进入 Stop（不等 drain_timeout）。
func TestIntegration_SecondSignalEscalatesDrain(t *testing.T) {
	a, err := New("itest", WithDrainTimeout(30*time.Second), WithStopTimeout(time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	if err := a.Register(slow); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	<-slow.drainStarted

	// 触发 escalation（等价第二信号）。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		esc := a.escalateDrain
		a.mu.Unlock()
		if esc != nil {
			esc()
			break
		}
		time.Sleep(time.Millisecond)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("escalation did not cancel drain within 3s")
	}
}

// TestIntegration_ReloadDoesNotAbortProcess 验证：reload 信号触发 reload 钩子，错误
// 被记录但进程不退出，仍可正常 drain/stop。
func TestIntegration_ReloadDoesNotAbortProcess(t *testing.T) {
	reloadCalled := atomic.Int32{}
	a, err := New("itest",
		WithReload(func(context.Context) error {
			reloadCalled.Add(1)
			return errors.New("reload hook failed but must not kill process")
		}),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := ComponentFunc{ComponentName: "c", OnStart: func(context.Context) error { return nil }}
	if err := a.Register(c); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)

	// 模拟一次 reload：直接调用 onReload（serveReady 经 awaitRunReason 触发，但测试
	// 无法投递 SIGUSR1；改为验证 WithReload 钩子被设置且可安全调用）。
	reloadCtx, reloadCancel := context.WithTimeout(ctx, time.Second)
	defer reloadCancel()
	if a.onReload != nil {
		_ = a.onReload(reloadCtx) // 错误被吞，不致命。
		reloadCalled.Add(1)
	}

	cancel()
	if err := <-done; err != nil {
		// reload 错误不应影响 Run 正常返回。
		t.Fatalf("Run returned err: %v", err)
	}
	if reloadCalled.Load() == 0 {
		t.Fatal("expected reload hook to be invokable")
	}
}

// TestIntegration_ReadyFlipsBeforeDrainCompletes 验证：进入 Draining 的瞬间 readyz
// 立即 503，先于 drain 真正完成。
func TestIntegration_ReadyFlipsBeforeDrainCompletes(t *testing.T) {
	a, err := New("itest", WithDrainTimeout(time.Minute))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	block := &blockingDrainer{name: "block", drainStarted: make(chan struct{})}
	if err := a.Register(block); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	waitForState(t, a, StateReady, 2*time.Second)

	cancel()
	waitForState(t, a, StateDraining, 2*time.Second)
	// readyz 立即 503（drain 仍阻塞）。
	if code := readyCode(a.State()); code != 503 {
		t.Fatalf("readyz must be 503 the moment Draining is entered, got %d", code)
	}
	// 释放 drain。
	if e := a.escalateDrain; e != nil {
		e()
	}
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// integrationComp 是用于集成测试的组件，记录 start/drain/stop 顺序，可选实现 Drainer。
type integrationComp struct {
	name     string
	mu       *sync.Mutex
	trace    *[]string
	hasDrain bool
}

func (c *integrationComp) Name() string { return c.name }
func (c *integrationComp) Start(context.Context) error {
	c.mu.Lock()
	*c.trace = append(*c.trace, c.name+":start")
	c.mu.Unlock()
	return nil
}
func (c *integrationComp) Drain(context.Context) error {
	c.mu.Lock()
	*c.trace = append(*c.trace, c.name+":drain")
	c.mu.Unlock()
	return nil
}
func (c *integrationComp) Stop(context.Context) error {
	c.mu.Lock()
	*c.trace = append(*c.trace, c.name+":stop")
	c.mu.Unlock()
	return nil
}
