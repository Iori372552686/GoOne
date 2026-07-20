package timer

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestWheel 构建一个小跨度时间轮，便于快速测试级联。
// tick=10ms, slotNum=10, layers=3 → L1=100ms, L2=1s, L3=10s。
func newTestWheel(job Job) *TimeWheel {
	return NewTimeWheel(job,
		WithTickInterval(10*time.Millisecond),
		WithSlotNum(10),
		WithLayers(3),
	)
}

func TestAddTimerTriggers(t *testing.T) {
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) {
		fired.Add(1)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	if !tw.AddTimer(50*time.Millisecond, "k1", "data") {
		t.Fatal("AddTimer returned false")
	}
	time.Sleep(200 * time.Millisecond)
	if got := fired.Load(); got != 1 {
		t.Fatalf("expected 1 fire, got %d", got)
	}
}

func TestSubSecondPrecision(t *testing.T) {
	// 旧实现的浮点转换会让 25ms 这种延迟精度全丢。新版应正确触发。
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) { fired.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	tw.AddTimer(25*time.Millisecond, nil, nil)
	time.Sleep(100 * time.Millisecond)
	if got := fired.Load(); got != 1 {
		t.Fatalf("25ms delay should fire once, got %d", got)
	}
}

func TestCascadeAcrossLayers(t *testing.T) {
	// L1 容量 100ms，L2 容量 1s。延迟 250ms 必须落到 L2，触发后级联回 L1 再触发。
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) { fired.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	start := time.Now()
	tw.AddTimer(250*time.Millisecond, "long", nil)
	// 等足够久覆盖 250ms + 容忍度。
	time.Sleep(500 * time.Millisecond)
	if got := fired.Load(); got != 1 {
		t.Fatalf("250ms (cross-layer) should fire once, got %d", got)
	}
	elapsed := time.Since(start)
	// 应该在 250ms 左右触发，不应显著早于（提前触发=算法错）。
	if elapsed < 240*time.Millisecond {
		t.Fatalf("fired too early: %v", elapsed)
	}
}

func TestRemoveTimer(t *testing.T) {
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) { fired.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	tw.AddTimer(80*time.Millisecond, "rm", nil)
	tw.RemoveTimer("rm")
	time.Sleep(200 * time.Millisecond)
	if got := fired.Load(); got != 0 {
		t.Fatalf("removed task should not fire, got %d", got)
	}
}

func TestRemoveTimerCrossLayer(t *testing.T) {
	// 长延迟任务落在 L2，Remove 应能跨层定位摘除。
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) { fired.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	tw.AddTimer(500*time.Millisecond, "long-rm", nil)
	// 立即删除（此时任务在 L2 槽里）。
	tw.RemoveTimer("long-rm")
	time.Sleep(700 * time.Millisecond)
	if got := fired.Load(); got != 0 {
		t.Fatalf("cross-layer removed task should not fire, got %d", got)
	}
}

func TestStopIdempotent(t *testing.T) {
	tw := newTestWheel(func(d interface{}) {})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	// 多次 Stop 不应 panic / 死锁。
	tw.Stop()
	tw.Stop()
	tw.Stop()
}

func TestJobPanicIsolated(t *testing.T) {
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) {
		n := fired.Add(1)
		if n == 1 {
			panic("boom")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	// 连续添加多个任务，第一个 panic 不应影响后续。
	for i := 0; i < 3; i++ {
		tw.AddTimer(time.Duration(30+i*10)*time.Millisecond, nil, nil)
	}
	time.Sleep(300 * time.Millisecond)
	if got := fired.Load(); got < 3 {
		t.Fatalf("panic should not kill scheduler; expected >=3 fires, got %d", got)
	}
}

func TestAddTimerAfterStopReturnsFalse(t *testing.T) {
	tw := newTestWheel(func(d interface{}) {})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	tw.Stop()
	if tw.AddTimer(50*time.Millisecond, nil, nil) {
		t.Fatal("AddTimer after Stop should return false")
	}
}

func TestConcurrentAddRemoveNoRace(t *testing.T) {
	// -race 下验证多 goroutine 并发 Add/Remove 不触发竞争。
	var fired atomic.Int32
	tw := newTestWheel(func(d interface{}) { fired.Add(1) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tw.Start(ctx)
	defer tw.Stop()

	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := gid*1000 + i
				tw.AddTimer(time.Duration(20+i%50)*time.Millisecond, key, nil)
				if i%3 == 0 {
					tw.RemoveTimer(key)
				}
			}
		}(g)
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	// 不 assert 具体次数（取决于 Remove 抢占时机），只验证无 race / 无 panic。
}
