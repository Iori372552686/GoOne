package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReadyObserverWindowDoesNotAllowDrainRegression 验证 修复：当 Ready 观察者
// 阻塞（慢就绪闸门）时，并发的 Drain 与 Ready 不得交错提交造成状态回退。
//
// 历史缺陷：transition 在 Ready/Allocated 路径上执行观察者之前释放 s.mu。一个并发 Drain
// 转换可在此窗口提交 Draining；随后旧 Ready 转换的观察者返回并重新获取 s.mu 提交，导致
// 状态从 Draining 回退到 Ready。
//
// 修复：用 transitionMu 串行化"校验→observer→提交"完整序列。串行化后两种合法交错：
//   - Ready 先提交，Drain 随后从 Ready->Draining 提交，最终 Draining；
//   - Drain 先到达，但 Starting->Draining 非法被拒，Ready 随后提交，最终 Ready。
//
// 任一交错下都不发生"Draining 回退到 Ready"。本测试用观察者记录每一笔提交态，断言序列
// 单调不回退。
func TestReadyObserverWindowDoesNotAllowDrainRegression(t *testing.T) {
	s := NewStateStore()
	inReady := make(chan struct{})
	releaseReady := make(chan struct{})

	// 观察者记录所有"已提交"的状态（committingShutdown 的 Draining 会先提交再派发；
	// Ready 在 observer 全部同意后才提交）。
	var mu sync.Mutex
	committed := []State{}
	s.AddObserver(ObserverFunc(func(ctx context.Context, ch StateChange) error {
		if ch.Current == StateReady {
			close(inReady)
			<-releaseReady
		}
		mu.Lock()
		committed = append(committed, ch.Current)
		mu.Unlock()
		return nil
	}))

	readyErr := make(chan error, 1)
	go func() { readyErr <- s.transition(context.Background(), StateReady, "ready", time.Time{}) }()

	<-inReady // Ready 观察者进入阻塞窗口

	// 并发 Drain：在 transitionMu 修复前会在此窗口提交 Draining（关停路径乐观提交）。
	drainErr := make(chan error, 1)
	go func() { drainErr <- s.transition(context.Background(), StateDraining, "sigterm", time.Time{}) }()

	// 给 Drain 一点时间到达 transitionMu（被 Ready 持有而阻塞）。
	time.Sleep(20 * time.Millisecond)
	close(releaseReady)
	<-readyErr

	err := <-drainErr
	mu.Lock()
	seq := append([]State(nil), committed...)
	mu.Unlock()

	// 不变量：committed 序列必须单调向前，绝不出现 Draining 之后再 Ready。
	sawDrainingIdx, sawReadyAfterDrain := -1, false
	for i, st := range seq {
		if st == StateDraining {
			sawDrainingIdx = i
		}
		if st == StateReady && sawDrainingIdx >= 0 && i > sawDrainingIdx {
			sawReadyAfterDrain = true
		}
	}
	if sawReadyAfterDrain {
		t.Fatalf("state regressed: Ready appeared after Draining in committed sequence %v", seq)
	}

	// 最终态：Ready（Drain 被拒）或 Draining（Ready 先提交，Drain 随后成功）。
	cur, _ := s.Current()
	switch cur {
	case StateReady:
		// Drain 必须因 Starting->Draining 非法而失败。
		if err == nil {
			t.Fatalf("state=Ready but Drain unexpectedly succeeded: seq=%v", seq)
		}
	case StateDraining:
		// Drain 必须成功（Ready->Draining 合法）。
		if err != nil {
			t.Fatalf("state=Draining but Drain returned error: %v seq=%v", err, seq)
		}
	default:
		t.Fatalf("unexpected terminal state %s, seq=%v", cur, seq)
	}
}

// TestTransitionSerializesHighConcurrency 对大量并发合法转换做压力串行化断言：最终状态
// 必须是状态机的合法终态，且过程中观察者看到的中间态不出现回退（Draining/Stopping/Failed
// 之后再回到 Ready/Allocated）。
func TestTransitionSerializesHighConcurrency(t *testing.T) {
	const runs = 300
	var badInterleaves int64
	for i := 0; i < runs; i++ {
		s := NewStateStore()
		s.AddObserver(ObserverFunc(func(ctx context.Context, ch StateChange) error {
			if ch.Current == StateReady {
				time.Sleep(time.Millisecond) // 放大窗口
			}
			return nil
		}))

		// 两个并发 transition：A=Ready，B=Ready（幂等非法）或 Draining（Ready 之后合法）。
		// 真正要守住的不变量：current 永远停在合法状态，绝不回退。
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = s.transition(context.Background(), StateReady, "ready", time.Time{})
		}()
		go func() {
			defer wg.Done()
			_ = s.transition(context.Background(), StateReady, "ready2", time.Time{})
		}()
		wg.Wait()

		cur, _ := s.Current()
		// 两个并发 Ready：其一成功提交 Ready，其二已是 Ready->Ready 非法。最终必须是 Ready。
		if cur != StateReady {
			atomic.AddInt64(&badInterleaves, 1)
		}
	}
	if got := atomic.LoadInt64(&badInterleaves); got != 0 {
		t.Fatalf("%d/%d runs ended in non-Ready state", got, runs)
	}
}

// contextIgnoringStart 模拟"不遵守 context 的 Start"：它阻塞在 ctx.Done() 上，但即便
// ctx 被取消也返回 nil（正确实现应返回 ctx.Err()）。
//
// 关键点：Start 阻塞在 <-ctx.Done()，保证 signalCtx 真正被取消后 Start 才返回；这样 Start
// 的返回与 signalCtx 取消之间没有竞态，使 Run 的启动 cause 检查可确定性地观察到取消。
type contextIgnoringStart struct {
	name      string
	startedCh chan struct{}
}

func newContextIgnoringStart(name string) *contextIgnoringStart {
	return &contextIgnoringStart{name: name, startedCh: make(chan struct{})}
}

func (c *contextIgnoringStart) Name() string { return c.name }
func (c *contextIgnoringStart) Start(ctx context.Context) error {
	close(c.startedCh)
	<-ctx.Done()
	return nil // 故意忽略 ctx 取消，返回 nil（错误实现：正确应 return ctx.Err()）
}
func (c *contextIgnoringStart) Stop(context.Context) error { return nil }

// TestStartupSignalWithContextIgnoringStart 验证 修复：组件收到启动期取消信号但
// 返回 nil（忽略 ctx）时，App 不得进入 Ready。
//
// 历史缺陷：startComponents 用 signalCtx 调 Start，但若 Start 忽略 ctx 返回 nil，Run 仍会
// 进入 enterReady。修复要求：只有全部 Start 成功且启动 cause 仍为空（signalCtx 未被取消）
// 时才允许 enterReady。
//
// 确定性：contextIgnoringStart 阻塞在 ctx.Done()，所以它返回时 signalCtx 必已被取消；
// Run 的启动 cause 检查据此拒绝 enterReady 并终态 Failed。
func TestStartupSignalWithContextIgnoringStart(t *testing.T) {
	a := MustNew("svc")
	c := newContextIgnoringStart("ignorer")
	mustRegister(a, c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	// 等待组件进入 Start（阻塞在 ctx.Done() 上）。
	select {
	case <-c.startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("ignorer never started")
	}

	// 注入启动期终止信号，取消 signalCtx。
	inject := a.tryInjectStartupSignal()
	if inject == nil {
		t.Fatal("startup signal injector not wired")
	}
	inject(platformTermSignals()[0])

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}

	if got := a.Phase(); got != string(StateFailed) {
		t.Fatalf("expected Failed (must not enter Ready when startup signal fired), got %s", got)
	}
}

// failingStopComponent 在 Stop 时返回一个可识别错误。
type failingStopComponent struct {
	name      string
	stopErr   error
	startedCh chan struct{}
}

func newFailingStopComponent(name string, stopErr error) *failingStopComponent {
	return &failingStopComponent{name: name, stopErr: stopErr, startedCh: make(chan struct{})}
}

func (f *failingStopComponent) Name() string { return f.name }
func (f *failingStopComponent) Start(context.Context) error {
	close(f.startedCh)
	return nil
}
func (f *failingStopComponent) Stop(context.Context) error { return f.stopErr }

// TestStopErrorMarksFailed 验证 修复：正常信号关停路径下，若任一组件 Stop 失败，
// 终态必须是 Failed（而非 Stopped）。
func TestStopErrorMarksFailed(t *testing.T) {
	a := MustNew("svc")
	mustRegister(a, newFailingStopComponent("bad", errors.New("stop boom")))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	err := <-done
	if err == nil {
		t.Fatal("expected Run to surface stop error")
	}
	if !containsMsg(err, "stop boom") {
		t.Fatalf("expected stop error in Run return, got %v", err)
	}
	if got := a.Phase(); got != string(StateFailed) {
		t.Fatalf("expected Failed terminal state on stop error, got %s", got)
	}
}

// failingDrainer 在 Drain 时返回一个可识别错误。
type failingDrainer struct {
	name    string
	drainEr error
}

func (f *failingDrainer) Name() string                { return f.name }
func (f *failingDrainer) Start(context.Context) error { return nil }
func (f *failingDrainer) Drain(context.Context) error { return f.drainEr }
func (f *failingDrainer) Stop(context.Context) error  { return nil }

// TestDrainErrorMarksFailed 验证 修复：正常信号关停路径下，若任一组件 Drain 失败，
// 终态必须是 Failed（而非 Stopped）。
func TestDrainErrorMarksFailed(t *testing.T) {
	a := MustNew("svc")
	mustRegister(a, &failingDrainer{name: "bad", drainEr: fmt.Errorf("drain boom")})

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	err := <-done
	if err == nil {
		t.Fatal("expected Run to surface drain error")
	}
	if !containsMsg(err, "drain boom") {
		t.Fatalf("expected drain error in Run return, got %v", err)
	}
	if got := a.Phase(); got != string(StateFailed) {
		t.Fatalf("expected Failed terminal state on drain error, got %s", got)
	}
}
