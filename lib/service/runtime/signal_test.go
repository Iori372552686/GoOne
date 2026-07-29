package runtime

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestSignalSourceRequiresSecondDistinctTermSignal 验证修复后的核心不变量：
// 第一次终止信号只触发 termCh（进入排空），secondCh 必须保持未关闭；只有第二次
// 终止信号才关闭 secondCh（强制取消排空）。
//
// 历史缺陷：installSignals 曾用两个 signal.Notify channel 同时订阅 SIGINT/SIGTERM，
// os/signal 会把每个信号实例投递给所有已注册 channel，导致第一次信号就同时进入
// termCh 与 secondCh，使 Drain 在开始的同时被 escalation 取消。
//
// 测试直接向 dispatcher 的原始信号 channel 注入信号，避免依赖 OS 自投递（在
// Windows 上不可靠）。
func TestSignalSourceRequiresSecondDistinctTermSignal(t *testing.T) {
	src := installSignals()
	defer src.stop()

	termSignals := platformTermSignals()

	// 注入第一次信号。
	src.injectForTest(termSignals[0])

	select {
	case <-src.termCh:
		// 第一次信号应当只进入 termCh。
	case <-time.After(time.Second):
		t.Fatal("第一次终止信号未投递到 termCh")
	}

	// secondCh 此时绝不能关闭。
	select {
	case <-src.secondCh:
		t.Fatal("第一次终止信号不应关闭 secondCh（会错误地立即 escalation）")
	default:
	}

	// 注入第二次信号（计数到 2 才触发 escalation）。
	src.injectForTest(termSignals[len(termSignals)-1])

	select {
	case <-src.secondCh:
		// 第二次信号关闭 secondCh，触发 escalation。
	case <-time.After(time.Second):
		t.Fatal("第二次终止信号未关闭 secondCh")
	}
}

// TestSignalSourceIgnoresReloadInTermDispatcher 确保重载信号不计入终止计数器：
// 只有 termSignals 子集中的信号才推进 first/second 计数。
func TestSignalSourceIgnoresReloadInTermDispatcher(t *testing.T) {
	_, reloadSignals := platformSignals()
	if len(reloadSignals) == 0 {
		t.Skip("本平台无重载信号")
	}
	src := installSignals()
	defer src.stop()

	// 重载信号不应进入 termCh 或 secondCh。
	src.injectForTest(reloadSignals[0])
	select {
	case <-src.termCh:
		t.Fatal("重载信号不应进入 termCh")
	case <-src.secondCh:
		t.Fatal("重载信号不应关闭 secondCh")
	case <-time.After(50 * time.Millisecond):
		// 预期：dispatcher 忽略非终止信号。
	}
}

// TestAwaitRunReasonReturnsTerminatedOnFirstSignal 确保首次信号归类为 "terminated"
// 而非触发任何提前的排空取消。
func TestAwaitRunReasonReturnsTerminatedOnFirstSignal(t *testing.T) {
	src := installSignals()
	defer src.stop()

	termSignals := platformTermSignals()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(20 * time.Millisecond)
		src.injectForTest(termSignals[0])
	}()

	reason, err := awaitRunReason(ctx, src)
	if reason != "terminated" {
		t.Fatalf("expected reason=terminated, got %q (err=%v)", reason, err)
	}
	if err != nil {
		t.Fatalf("terminated 不应携带 ctx 错误，got %v", err)
	}
}

// TestDrainTimeoutRecordsDeadlineExceeded 验证：Drain 真实超时（时间耗尽）时，
// Run 返回的 joined error 链包含 context.DeadlineExceeded，
// goone_drain_timeouts_total 计数增加 1，且 Stop 仍被调用。
//
// 历史缺陷：drainComponents 曾用 context.WithCancel + 一个 goroutine 调 drainCancel
// 来模拟超时，导致 drainCtx.Err() 返回 context.Canceled 而非 DeadlineExceeded，
// 从而 timedOut 判定永远为 false、drain_timeouts_total 永不增长。
func TestDrainTimeoutRecordsDeadlineExceeded(t *testing.T) {
	before := readDrainTimeoutsMetric()

	a, _, _ := newTraceApp(t, "svc",
		WithDrainTimeout(40*time.Millisecond),
		WithStopTimeout(time.Second),
	)
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	mustRegister(a, slow)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	err := <-done

	if err == nil {
		t.Fatal("期望 Run 返回非空错误（drain 超时）")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 error 链包含 DeadlineExceeded，got %v", err)
	}
	if got := readDrainTimeoutsMetric() - before; got != 1 {
		t.Fatalf("期望 drain_timeouts_total 增加 1，增加量=%d", got)
	}
	if !slow.stopped.Load() {
		t.Fatal("超时后 Stop 必须仍被调用")
	}
}

// TestDrainEscalationReturnsErrDrainEscalated 验证：第二次终止信号（升级）取消
// 排空时，Run 返回的 error 链包含 ErrDrainEscalated，且不增加超时计数
// （升级 != 超时）。
func TestDrainEscalationReturnsErrDrainEscalated(t *testing.T) {
	before := readDrainTimeoutsMetric()

	a, _, _ := newTraceApp(t, "svc",
		WithDrainTimeout(30*time.Second),
		WithStopTimeout(time.Second),
	)
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	mustRegister(a, slow)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	<-slow.drainStarted

	waitForEscalateAndFire(t, a)

	err := <-done
	if err == nil {
		t.Fatal("期望 Run 返回非空错误（升级路径）")
	}
	if !errors.Is(err, ErrDrainEscalated) {
		t.Fatalf("期望 error 链包含 ErrDrainEscalated，got %v", err)
	}
	if got := readDrainTimeoutsMetric() - before; got != 0 {
		t.Fatalf("升级不应增加 drain_timeouts_total，增加量=%d", got)
	}
	if !slow.stopped.Load() {
		t.Fatal("升级后 Stop 必须仍被调用")
	}
}

// --- helpers ---

func waitForEscalateAndFire(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if esc := a.tryEscalateDrain(); esc != nil {
			esc()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("escalateDrain 未在超时内接线")
}

// 防止未用导入（os 仅用于文档化的信号类型引用）。
var _ os.Signal = nil
