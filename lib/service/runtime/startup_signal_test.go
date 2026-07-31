package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// slowStartComponent 在 Start 中阻塞直到 ctx 被取消，用于测试启动期信号能否中断
// 进行中的 Component.Start。
type slowStartComponent struct {
	name      string
	startedCh chan struct{} // Start 被调用时关闭
	stoppedCh chan struct{} // Stop 被调用时关闭
}

func newSlowStartComponent(name string) *slowStartComponent {
	return &slowStartComponent{
		name:      name,
		startedCh: make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

func (s *slowStartComponent) Name() string { return s.name }

func (s *slowStartComponent) Start(ctx context.Context) error {
	close(s.startedCh)
	<-ctx.Done() // 阻塞直到 signalCtx 被启动期信号取消
	return ctx.Err()
}

func (s *slowStartComponent) Stop(ctx context.Context) error {
	select {
	case <-s.stoppedCh:
	default:
		close(s.stoppedCh)
	}
	return nil
}

// TestStartupSignalCancelsStartComponents 验证启动阶段收到终止信号时，
// signalCtx 被取消，进行中的 Component.Start 立即返回，已成功启动的组件逆序 Stop，
// Run 返回 error 且终态 Failed。
func TestStartupSignalCancelsStartComponents(t *testing.T) {
	a := MustNew("svc")
	// 第一个组件正常启动；第二个是慢 Start，会被信号中断。
	var mu sync.Mutex
	trace := []string{}
	first := newRecordingComponent("first", &trace, &mu)
	mustRegister(a, first)
	slow := newSlowStartComponent("slow")
	mustRegister(a, slow)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	// 等待慢组件进入 Start（说明 first 已启动，正在启动 slow）。
	select {
	case <-slow.startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("slow component never started")
	}

	// 注入第一个终止信号（模拟启动期 SIGTERM）。
	inject := a.tryInjectStartupSignal()
	if inject == nil {
		t.Fatal("startup signal injector not wired (Run not yet past installSignals?)")
	}
	inject(platformTermSignals()[0])

	// Run 应在启动期信号后返回 error：signalCtx 被取消使慢组件 Start 返回
	// context.Canceled，startComponents 据此回滚。
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected Run to return error on startup signal")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled from interrupted start, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after startup signal")
	}

	// 终态应为 Failed。
	if a.Phase() != string(StateFailed) {
		t.Fatalf("expected terminal state Failed, got %s", a.Phase())
	}

	// 已成功启动的 first 组件必须被回滚 Stop（slow 的 Start 失败，不在回滚列表）。
	mu.Lock()
	got := append([]string(nil), trace...)
	mu.Unlock()
	stopped := false
	for _, e := range got {
		if e == "first:stop" {
			stopped = true
		}
	}
	if !stopped {
		t.Fatalf("expected first component to be rolled back (stopped); trace=%v", got)
	}
}

// TestStartupSignalStillAllowsSecondSignalEscalation 验证启动期信号前置后，
// 第二次终止信号的升级路径（secondCh）仍然有效。
func TestStartupSignalStillAllowsSecondSignalEscalation(t *testing.T) {
	// 用 installSignals 直接验证 dispatcher 计数不受前置影响。
	src := installSignals()
	defer src.stop()

	term := platformTermSignals()[0]
	src.injectForTest(term) // 第一次信号 → termCh
	select {
	case <-src.termCh:
	case <-time.After(time.Second):
		t.Fatal("first signal did not reach termCh")
	}

	// 第二次信号应触发 escalate（关闭 secondCh）。
	escFired := make(chan struct{})
	go func() {
		select {
		case <-src.secondCh:
			close(escFired)
		case <-time.After(time.Second):
		}
	}()
	src.injectForTest(term) // 第二次信号 → escalate
	select {
	case <-escFired:
	case <-time.After(2 * time.Second):
		t.Fatal("second signal did not trigger escalation")
	}
}

// runtimeErrComponent 是实现 RuntimeErrorSource 的测试组件，发送一个可识别的 error。
type runtimeErrComponent struct {
	name   string
	errCh  chan error
	started chan struct{}
}

func newRuntimeErrComponent(name string) *runtimeErrComponent {
	return &runtimeErrComponent{name: name, errCh: make(chan error, 1), started: make(chan struct{})}
}

func (r *runtimeErrComponent) Name() string { return r.name }
func (r *runtimeErrComponent) Start(ctx context.Context) error {
	close(r.started)
	return nil
}
func (r *runtimeErrComponent) Stop(ctx context.Context) error { return nil }
func (r *runtimeErrComponent) RuntimeErrors() <-chan error    { return r.errCh }

// TestRuntimeErrorWrappedWithComponentName 验证 RuntimeErrorSource 组件上报的运行期
// 错误在 Run 返回值中带组件名（错误中包含组件名）。
func TestRuntimeErrorWrappedWithComponentName(t *testing.T) {
	a := MustNew("svc")
	mustRegister(a, newRecordingComponent("dep", &[]string{}, &sync.Mutex{}))
	rc := newRuntimeErrComponent("web")
	mustRegister(a, rc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	// 等待组件进入 Ready。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.Phase() == string(StateReady) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	<-rc.started

	// 触发运行期错误。
	rc.errCh <- errors.New("listener died")

	// Run 应返回带组件名 "web" 的 error，终态 Failed。
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected Run to return runtime error")
		}
		if !strings.Contains(err.Error(), `"web"`) {
			t.Fatalf("expected error to contain component name %q, got: %v", "web", err)
		}
		// 包装的 error 应保留原始根因文本。
		if !strings.Contains(err.Error(), "listener died") {
			t.Fatalf("expected wrapped error to contain root cause, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after runtime error")
	}

	if a.Phase() != string(StateFailed) {
		t.Fatalf("expected terminal state Failed, got %s", a.Phase())
	}
}
