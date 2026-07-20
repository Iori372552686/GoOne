package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestTaskRunOnStart(t *testing.T) {
	var calls atomic.Int64
	task := New("t", 0, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	task.RunOnStart = true
	// Interval<=0 且 RunOnStart=true：仅立即跑一次，无周期。
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// RunOnStart 异步执行，等其完成（Stop 会 join loop）。
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

func TestTaskNoOpWhenNoIntervalAndNoRunOnStart(t *testing.T) {
	task := New("noop", 0, func(context.Context) error { return nil })
	// 不应启动 goroutine，Start/Stop 立即返回。
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestTaskPeriodic(t *testing.T) {
	var calls atomic.Int64
	task := New("periodic", 20*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return nil
	})
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// 等 ~5 个周期。
	time.Sleep(120 * time.Millisecond)
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := calls.Load(); got < 3 {
		t.Fatalf("expected >=3 calls in 120ms @20ms, got %d", got)
	}
}

func TestTaskNonOverlapSkips(t *testing.T) {
	// Run 阻塞超过一个周期，应触发 NonOverlap 跳过。
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	task := New("slow", 15*time.Millisecond, func(ctx context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return ctx.Err()
	})
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-started // 第一次进入并持锁阻塞。
	// 等 3 个周期，期间后续触发应被 NonOverlap 跳过。
	time.Sleep(60 * time.Millisecond)
	if got := task.Skipped(); got < 1 {
		t.Fatalf("expected >=1 skipped, got %d", got)
	}
	close(release)
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestTaskStopWaitsForRun(t *testing.T) {
	inRun := make(chan struct{})
	done := make(chan struct{})
	task := New("wait", time.Second, func(ctx context.Context) error {
		close(inRun)
		<-ctx.Done()
		close(done)
		return ctx.Err()
	})
	task.RunOnStart = true // 立即进入 Run。
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	<-inRun
	// Stop 应等 Run 退出。
	stopDone := make(chan error, 1)
	go func() { stopDone <- task.Stop(context.Background()) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not let Run observe cancellation in time")
	}
	select {
	case err := <-stopDone:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after Run finished")
	}
}

func TestTaskPanicDoesNotKillLoop(t *testing.T) {
	var calls atomic.Int64
	task := New("panicky", 15*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		if calls.Load() == 1 {
			panic("boom")
		}
		return nil
	})
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// 第一次 panic 后 loop 应继续，后续周期仍执行。
	if got := calls.Load(); got < 2 {
		t.Fatalf("expected loop to survive panic and run >=2 times, got %d", got)
	}
}

func TestTaskRunErrorLoggedNotFatal(t *testing.T) {
	var calls atomic.Int64
	task := New("err", 15*time.Millisecond, func(context.Context) error {
		n := calls.Add(1)
		if n == 1 {
			return errors.New("transient")
		}
		return nil
	})
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := calls.Load(); got < 2 {
		t.Fatalf("expected >=2 calls after a returned error, got %d", got)
	}
}

func TestTaskStopIsIdempotent(t *testing.T) {
	task := New("idem", 10*time.Millisecond, func(context.Context) error { return nil })
	if err := task.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop 1: %v", err)
	}
	if err := task.Stop(context.Background()); err != nil {
		t.Fatalf("Stop 2 must be idempotent: %v", err)
	}
}
