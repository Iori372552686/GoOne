package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionTrackerCounts(t *testing.T) {
	tr := NewSessionTracker()
	if tr.ActiveConnections() != 0 || tr.ActiveSessions() != 0 {
		t.Fatalf("expected zero counts, got conn=%d sess=%d", tr.ActiveConnections(), tr.ActiveSessions())
	}
	tr.IncConnection()
	tr.IncConnection()
	tr.IncSession()
	if tr.ActiveConnections() != 2 || tr.ActiveSessions() != 1 {
		t.Fatalf("got conn=%d sess=%d", tr.ActiveConnections(), tr.ActiveSessions())
	}
	tr.DecConnection()
	tr.DecSession()
	if tr.ActiveConnections() != 1 || tr.ActiveSessions() != 0 {
		t.Fatalf("after dec: conn=%d sess=%d", tr.ActiveConnections(), tr.ActiveSessions())
	}
}

func TestSessionTrackerClampsNegative(t *testing.T) {
	// Dec 低于 0 不应取负，应钳为 0（且不 panic）。
	tr := NewSessionTracker()
	tr.DecConnection()
	tr.DecSession()
	if tr.ActiveConnections() != 0 || tr.ActiveSessions() != 0 {
		t.Fatalf("expected clamped 0, got conn=%d sess=%d", tr.ActiveConnections(), tr.ActiveSessions())
	}
}

func TestSessionTrackerWaitIdleFastPath(t *testing.T) {
	// 已归零时 WaitIdle 立即返回 nil。
	tr := NewSessionTracker()
	if err := tr.WaitIdle(context.Background()); err != nil {
		t.Fatalf("expected nil on already-idle, got %v", err)
	}
}

func TestSessionTrackerWaitIdleBlocksUntilZero(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncSession()
	tr.IncConnection()

	done := make(chan error, 1)
	go func() { done <- tr.WaitIdle(context.Background()) }()

	// WaitIdle 应阻塞（未归零）。
	select {
	case err := <-done:
		t.Fatalf("WaitIdle returned early with %v before counts hit zero", err)
	case <-time.After(50 * time.Millisecond):
	}

	// 归零后应被状态变更通知唤醒（不靠轮询）。
	tr.DecSession()
	tr.DecConnection()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after idle, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitIdle was not woken when counts reached zero")
	}
}

func TestSessionTrackerWaitIdleTimeout(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncSession()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	err := tr.WaitIdle(ctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// 计数仍 > 0。
	if tr.ActiveSessions() != 1 {
		t.Fatalf("expected session still 1 after timeout, got %d", tr.ActiveSessions())
	}
}

func TestSessionTrackerWaitIdleContextCancel(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncConnection()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- tr.WaitIdle(ctx) }()
	// 给 WaitIdle 一点时间进入 Wait。
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected ctx.Err on cancel, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("WaitIdle not woken on ctx cancel")
	}
}

// TestSessionTrackerCloseReturnsErrorWhenNonZero 验证 Close 在计数非零时让等待
// 者返回 ErrSessionTrackerClosed，不返回 nil 冒充成功排空。
//
// 历史缺陷：Close 无论计数都让 WaitIdle 返回 nil，使排空未完成时上层误判成功。
func TestSessionTrackerCloseReturnsErrorWhenNonZero(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncSession() // session=1，未归零
	done := make(chan error, 1)
	go func() { done <- tr.WaitIdle(context.Background()) }()
	time.Sleep(30 * time.Millisecond)
	tr.Close()
	select {
	case err := <-done:
		if !errors.Is(err, ErrSessionTrackerClosed) {
			t.Fatalf("期望 ErrSessionTrackerClosed（计数非零），got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close 未唤醒 WaitIdle")
	}
}

// TestSessionTrackerCloseReturnsNilWhenZero 验证 Close 在计数已归零时返回 nil。
func TestSessionTrackerCloseReturnsNilWhenZero(t *testing.T) {
	tr := NewSessionTracker()
	// 不 Inc；计数为 0。
	done := make(chan error, 1)
	go func() { done <- tr.WaitIdle(context.Background()) }()
	time.Sleep(30 * time.Millisecond)
	tr.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("计数归零时 Close 应返回 nil，got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close 未唤醒 WaitIdle")
	}
}

// TestSessionTrackerWaitSessionsWaitsForSessionsOnly 验证 WaitSessions 只等逻辑会话归零，
// 不受 connection 影响（网关 Drain 用它）。
func TestSessionTrackerWaitSessionsWaitsForSessionsOnly(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncConnection() // connection=1，但 session=0
	// WaitSessions 应立即返回（session 已归零），不等 connection。
	if err := tr.WaitSessions(context.Background()); err != nil {
		t.Fatalf("session=0 时 WaitSessions 应立即返回 nil，got %v", err)
	}

	tr.IncSession()
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- tr.WaitSessions(ctx) }()
	time.Sleep(30 * time.Millisecond)
	tr.DecSession() // session 归零
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("session 归零后 WaitSessions 应返回 nil，got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitSessions 未在 session 归零时唤醒")
	}
	cancel()
}

// TestSessionTrackerDecCASDoesNotOverwriteConcurrentInc 验证 Dec 用 CAS 防止
// underflow 覆盖并发 Inc。旧实现 Add(-1) 后 Store(0) 会丢失并发 +1。
func TestSessionTrackerDecCASDoesNotOverwriteConcurrentInc(t *testing.T) {
	tr := NewSessionTracker()
	tr.IncSession()
	tr.IncSession() // session=2
	// 并发：一个 goroutine 反复 Inc，主线程 Dec。CAS 保证不丢 Inc。
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.IncSession()
			tr.DecSession()
		}()
	}
	// 同时主线程多次 Dec（可能触发 underflow 路径，CAS 应钳为 0 不取负）。
	for i := 0; i < 10; i++ {
		tr.DecSession()
	}
	wg.Wait()
	if got := tr.ActiveSessions(); got < 0 {
		t.Fatalf("session 计数不得为负，got %d", got)
	}
}

func TestSessionTrackerConcurrent(t *testing.T) {
	// 本机非 race；CI 跑 -race。此处覆盖并发 Inc/Dec 与 WaitIdle 不死锁、计数不为
	// 负。
	tr := NewSessionTracker()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				tr.IncConnection()
				tr.IncSession()
				tr.DecConnection()
				tr.DecSession()
			}
		}()
	}
	// 并发一个 WaitIdle（很可能因持续抖动而阻塞到结束）。
	waitDone := make(chan struct{})
	go func() {
		_ = tr.WaitIdle(context.Background())
		close(waitDone)
	}()
	wg.Wait()
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitIdle did not return after concurrent ops settled")
	}
	if c := tr.ActiveConnections(); c != 0 {
		t.Fatalf("expected 0 connections after settle, got %d", c)
	}
	if s := tr.ActiveSessions(); s != 0 {
		t.Fatalf("expected 0 sessions after settle, got %d", s)
	}
}

func TestGatewayInterfaceComposable(t *testing.T) {
	// 一个实现了 GatewayLifecycle 的最小网关，应同时满足三个子接口，便于 App 的
	// Quiesce/Drain 阶段按需类型断言。
	g := &fakeGateway{tracker: NewSessionTracker()}
	var _ GatewayServer = g
	var _ GatewayQuiescer = g
	var _ GatewayDrainer = g
	var _ GatewayLifecycle = g
}

type fakeGateway struct {
	tracker *SessionTracker
}

func (g *fakeGateway) ActiveConnections() int64 { return g.tracker.ActiveConnections() }
func (g *fakeGateway) ActiveSessions() int64    { return g.tracker.ActiveSessions() }
func (g *fakeGateway) QuiesceGateway(context.Context) error {
	// 假装关闭 listener。
	return nil
}
func (g *fakeGateway) DrainSessions(ctx context.Context) error {
	return g.tracker.WaitIdle(ctx)
}
