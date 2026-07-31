package async

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestPushERejectsWhenStopped 验证 worker 未 Start 时 PushE 返回 ErrWorkerStopped。
func TestPushERejectsWhenStopped(t *testing.T) {
	a := NewAsync()
	if err := a.PushE(func() {}); !errors.Is(err, ErrWorkerStopped) {
		t.Fatalf("before start, err=%v want ErrWorkerStopped", err)
	}
}

// TestPushERejectsAfterQuiesce 验证 Quiesce 后 PushE 返回 ErrWorkerQuiescing。
func TestPushERejectsAfterQuiesce(t *testing.T) {
	a := NewAsync()
	a.Start()
	defer a.Stop()
	a.Quiesce()
	if err := a.PushE(func() {}); !errors.Is(err, ErrWorkerQuiescing) {
		t.Fatalf("after quiesce, err=%v want ErrWorkerQuiescing", err)
	}
}

// TestPushERejectsWhenQueueFull 验证队列满时 PushE 返回 ErrQueueFull。
// 用极小容量 + 阻塞任务填满队列触发。
func TestPushERejectsWhenQueueFull(t *testing.T) {
	a := NewAsyncWithCapacity(2)
	a.Start()
	defer a.Stop()
	// 投一个永不完成的任务，占住 worker，使后续任务堆积在队列。
	block := make(chan struct{})
	defer close(block)
	if err := a.PushE(func() { <-block }); err != nil {
		t.Fatalf("first push: %v", err)
	}
	// 等待 worker 取走第一个任务（队列空，worker 忙）。
	time.Sleep(20 * time.Millisecond)
	// 填满容量（2）。
	if err := a.PushE(func() {}); err != nil {
		t.Fatalf("push 1: %v", err)
	}
	if err := a.PushE(func() {}); err != nil {
		t.Fatalf("push 2: %v", err)
	}
	// 第三个应被拒绝（队列满）。
	if err := a.PushE(func() {}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("push 3 err=%v want ErrQueueFull", err)
	}
}

// TestDrainWaitsForQueueEmpty 验证 Drain 在队列归零后返回 nil。
func TestDrainWaitsForQueueEmpty(t *testing.T) {
	a := NewAsync()
	a.Start()
	defer a.Stop()
	var done int64
	for i := 0; i < 50; i++ {
		if err := a.PushE(func() { atomic.AddInt64(&done, 1) }); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
	}
	a.Quiesce()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.Drain(ctx); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := atomic.LoadInt64(&done); got != 50 {
		t.Fatalf("done=%d want 50", got)
	}
}

// TestDrainReturnsErrorOnCtxCancel 验证 ctx 取消时 Drain 返回 ctx.Err。
// 关键：Drain 等待的是队列深度归零，而非在途任务完成。因此需要让队列保持非空：
// worker 正在处理第一个阻塞任务时，第二个阻塞任务仍排在队列里。
func TestDrainReturnsErrorOnCtxCancel(t *testing.T) {
	a := NewAsyncWithCapacity(8)
	a.Start()
	defer a.Stop()
	block := make(chan struct{})
	defer close(block)
	// 第一个任务占住 worker（永不完成），第二个任务排在队列里保持非零深度。
	if err := a.PushE(func() { <-block }); err != nil {
		t.Fatalf("push 1: %v", err)
	}
	if err := a.PushE(func() { <-block }); err != nil {
		t.Fatalf("push 2: %v", err)
	}
	// 等 worker 取走第一个任务（队列里仍剩第二个，深度=1）。
	time.Sleep(30 * time.Millisecond)
	a.Quiesce()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := a.Drain(ctx); err == nil {
		t.Fatal("expected drain to return ctx error on cancel while queue non-empty")
	}
}

// TestPushDeprecatedWrapperNoError 验证旧 Push 签名仍可用（兼容，）。
func TestPushDeprecatedWrapperNoError(t *testing.T) {
	a := NewAsync()
	a.Start()
	defer a.Stop()
	var done int64
	a.Push(func() { atomic.AddInt64(&done, 1) })
	// 等待 worker 处理。
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for atomic.LoadInt64(&done) == 0 {
		select {
		case <-ctx.Done():
			t.Fatal("task never executed")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestWorkerComponentLifecycle 验证 WorkerComponent 的 Start/Quiesce/Drain/Stop。
func TestWorkerComponentLifecycle(t *testing.T) {
	pool := NewAsyncPoolWithCapacity(3, 64) // 大容量避免快任务下偶发 queue full
	wc := NewWorkerComponent("mysql_async", pool)
	if err := wc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	var done int64
	for i := 0; i < 30; i++ {
		if err := wc.Push(i%3, func() { atomic.AddInt64(&done, 1) }); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
	}
	if err := wc.Quiesce(context.Background()); err != nil {
		t.Fatalf("quiesce: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := wc.Drain(ctx); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := atomic.LoadInt64(&done); got != 30 {
		t.Fatalf("done=%d want 30", got)
	}
	if err := wc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
