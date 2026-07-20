package datetime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTickUpdatesSnapshot(t *testing.T) {
	// init() 已存初始快照。等一小段后 Tick，NowT 应推进。
	before := NowT()
	time.Sleep(20 * time.Millisecond)
	Tick()
	after := NowT()
	if !after.After(before) {
		t.Fatalf("Tick should advance snapshot: before=%v after=%v", before, after)
	}
}

func TestSetTickInterval(t *testing.T) {
	orig := TickInterval()
	defer SetTickInterval(orig)
	SetTickInterval(250 * time.Millisecond)
	if got := TickInterval(); got != 250*time.Millisecond {
		t.Fatalf("expected 250ms, got %v", got)
	}
	// 非正值被忽略。
	SetTickInterval(0)
	if got := TickInterval(); got != 250*time.Millisecond {
		t.Fatalf("zero should be ignored, got %v", got)
	}
}

func TestDefaultTickInterval(t *testing.T) {
	if DefaultTickInterval != 100*time.Millisecond {
		t.Fatalf("DefaultTickInterval should be 100ms, got %v", DefaultTickInterval)
	}
}

func TestOffset(t *testing.T) {
	orig := TimeOffset()
	defer SetTimeOffset(orig)
	SetTimeOffset(60)
	if got := TimeOffset(); got != 60 {
		t.Fatalf("expected offset 60, got %d", got)
	}
	// Now 含 offset，NowNoOffset 不含。
	if Now()-NowNoOffset() != 60 {
		t.Fatalf("Now - NowNoOffset should equal offset 60")
	}
	// NowInt64 含 offset。
	if NowInt64()-int64(NowNoOffset()) != 60 {
		t.Fatalf("NowInt64 should include offset")
	}
}

func TestConcurrentReadTickNoRace(t *testing.T) {
	// 即使没有 -race（环境缺 cgo），这个测试仍验证并发读写不会 panic/deadlock。
	// 多 goroutine 读 + 单 goroutine 高频 Tick。
	var stop atomic.Bool
	var wg sync.WaitGroup
	// 8 个读 goroutine。
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				_ = Now()
				_ = NowMs()
				_ = NowUs()
				_ = NowInt64()
				_ = NowT()
				_ = NowNoOffset()
			}
		}()
	}
	// 1 个写 goroutine。
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			Tick()
		}
		stop.Store(true)
	}()
	wg.Wait()
}

func TestGetDataFormats(t *testing.T) {
	// 仅验证格式不 panic 且非空。GetDataHMS 旧实现用了错误格式 "15:04:16"（秒位硬编码 16），
	// 新版修正为 "15:04:05"。
	d := GetData()
	hms := GetDataHMS()
	if len(d) != 10 {
		t.Fatalf("GetData should be YYYY-MM-DD (10 chars), got %q", d)
	}
	if len(hms) != 19 {
		t.Fatalf("GetDataHMS should be YYYY-MM-DD HH:MM:SS (19 chars), got %q", hms)
	}
}

func TestIsSameDay(t *testing.T) {
	t1 := int64(1700000000) // 2023-11-14 22:13:20 UTC
	// 同一秒必同一天。
	if !IsSameDay(t1, t1) {
		t.Fatal("same timestamp should be same day")
	}
}
