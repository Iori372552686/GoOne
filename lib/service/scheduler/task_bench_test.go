package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkTaskIdle 度量空闲 Task（周期内无实际工作）的调度开销。这是“空闲服务不再
// 每 10ms 唤醒”之后，单个精确周期 Task 的稳态成本基线。
func BenchmarkTaskIdle(b *testing.B) {
	task := New("bench_idle", 10*time.Millisecond, func(context.Context) error {
		return nil
	})
	if err := task.Start(context.Background()); err != nil {
		b.Fatalf("Start: %v", err)
	}
	b.ResetTimer()
	// 跑足够长的时间，让周期触发累计 b.N 次的成本被摊薄度量。
	time.Sleep(time.Duration(b.N) * time.Microsecond)
	b.StopTimer()
	_ = task.Stop(context.Background())
}

// BenchmarkTaskRun 度量 Task 单次 Run（含 trigger goroutine 启动 + NonOverlap 锁 +
// metrics 上报）的开销。与 BenchmarkTaskIdle 对照：本基准的 Run 有一次原子自增。
func BenchmarkTaskRun(b *testing.B) {
	var n atomic.Int64
	task := New("bench_run", time.Hour, func(context.Context) error {
		n.Add(1)
		return nil
	})
	// 直接度量 trigger→runOnce 路径（不经周期等待），用极长 Interval 避免周期触发
	// 污染，手动触发 b.N 次。
	if err := task.Start(context.Background()); err != nil {
		b.Fatalf("Start: %v", err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// trigger 是内部方法；这里通过 Stop 前的在途执行度量。为稳定度量，改为
		// 直接调用 runOnce（已导出给 benchmark 同包访问）。
		task.runOnce(ctx)
		// 等待 goroutine 完成（inFlight join）。
		task.inFlight.Wait()
	}
	b.StopTimer()
	_ = task.Stop(context.Background())
}

// BenchmarkTaskNonOverlapSkip 度量 NonOverlap 跳过路径（TryLock 失败）的开销。
func BenchmarkTaskNonOverlapSkip(b *testing.B) {
	task := New("bench_skip", time.Hour, func(context.Context) error {
		return nil
	})
	task.NonOverlap = true
	if err := task.Start(context.Background()); err != nil {
		b.Fatalf("Start: %v", err)
	}
	ctx := context.Background()
	// 先锁住 runMu，使后续 trigger 全部走跳过路径。
	task.runMu.Lock()
	defer task.runMu.Unlock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task.trigger(ctx)
	}
}
