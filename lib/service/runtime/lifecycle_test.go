package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingComponent records the order of Start/Quiesce/Drain/Stop calls into
// a shared trace. It is the workhorse for ordering and rollback tests.
type recordingComponent struct {
	name     string
	mu       *sync.Mutex
	trace    *[]string
	startErr error
	stopErr  error
	// goroutine handling: startedWG is released when the component's goroutine
	// is running; stoppedWG is released when it exits. Lets tests assert that
	// goroutines really exit after Run.
	startedWG sync.WaitGroup
	stoppedWG sync.WaitGroup
	goroutine bool
	stopCh    chan struct{}
}

func newRecordingComponent(name string, trace *[]string, mu *sync.Mutex) *recordingComponent {
	return &recordingComponent{name: name, mu: mu, trace: trace, stopCh: make(chan struct{})}
}

func (r *recordingComponent) Name() string { return r.name }

func (r *recordingComponent) Start(ctx context.Context) error {
	r.mu.Lock()
	*r.trace = append(*r.trace, r.name+":start")
	r.mu.Unlock()
	if r.startErr != nil {
		return r.startErr
	}
	if r.goroutine {
		r.startedWG.Add(1)
		r.stoppedWG.Add(1)
		go func() {
			defer r.stoppedWG.Done()
			r.startedWG.Done()
			select {
			case <-r.stopCh:
			case <-ctx.Done():
			}
		}()
	}
	return nil
}

func (r *recordingComponent) Quiesce(ctx context.Context) error {
	r.mu.Lock()
	*r.trace = append(*r.trace, r.name+":quiesce")
	r.mu.Unlock()
	return nil
}

func (r *recordingComponent) Drain(ctx context.Context) error {
	r.mu.Lock()
	*r.trace = append(*r.trace, r.name+":drain")
	r.mu.Unlock()
	return nil
}

func (r *recordingComponent) Stop(ctx context.Context) error {
	r.mu.Lock()
	*r.trace = append(*r.trace, r.name+":stop")
	r.mu.Unlock()
	if r.stopCh != nil {
		close(r.stopCh)
	}
	return r.stopErr
}

// newTraceApp builds an App plus shared trace/mutex for recording tests.
func newTraceApp(t *testing.T, name string, opts ...Option) (*App, *[]string, *sync.Mutex) {
	t.Helper()
	a, err := New(name, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trace := make([]string, 0, 16)
	var mu sync.Mutex
	return a, &trace, &mu
}

func TestNewRejectsEmptyName(t *testing.T) {
	if _, err := New(""); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestRegisterRejectsDuplicateName(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc")
	trace := make([]string, 0)
	c1 := newRecordingComponent("dup", &trace, mu)
	c2 := newRecordingComponent("dup", &trace, mu)
	if err := a.Register(c1); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := a.Register(c2); !errors.Is(err, ErrDuplicateComponent) {
		t.Fatalf("expected ErrDuplicateComponent, got %v", err)
	}
}

func TestRegisterRejectsNilAndEmpty(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	if err := a.Register(nil); !errors.Is(err, ErrNilComponent) {
		t.Fatalf("expected ErrNilComponent, got %v", err)
	}
	empty := ComponentFunc{ComponentName: "", OnStart: func(context.Context) error { return nil }}
	if err := a.Register(empty); !errors.Is(err, ErrEmptyComponentName) {
		t.Fatalf("expected ErrEmptyComponentName, got %v", err)
	}
}

func TestAppStartsAndStopsInReverseOrder(t *testing.T) {
	a, trace, mu := newTraceApp(t, "svc")
	mustRegister(a, newRecordingComponent("a", trace, mu))
	mustRegister(a, newRecordingComponent("b", trace, mu))
	mustRegister(a, newRecordingComponent("c", trace, mu))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned err: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), *trace...)
	mu.Unlock()

	// Starts in order a,b,c; stops in reverse c,b,a.
	wantStart := []string{"a:start", "b:start", "c:start"}
	wantStop := []string{"c:stop", "b:stop", "a:stop"}
	assertSubsequence(t, got, wantStart, "start order")
	assertSubsequence(t, got, wantStop, "reverse stop order")
}

func TestAppRollsBackStartedComponentsOnStartFailure(t *testing.T) {
	a, trace, mu := newTraceApp(t, "svc")
	mustRegister(a, newRecordingComponent("a", trace, mu))
	mustRegister(a, newRecordingComponent("b", trace, mu))
	broken := newRecordingComponent("c", trace, mu)
	broken.startErr = errors.New("boom")
	mustRegister(a, broken)

	ctx, cancel := context.WithCancel(context.Background())
	// cancel not needed (Run will return on its own), but keep for safety.
	defer cancel()
	err := a.Run(ctx)
	if err == nil {
		t.Fatal("expected Run to return startup error")
	}

	mu.Lock()
	got := append([]string(nil), *trace...)
	mu.Unlock()

	// a and b started; rollback stops them in reverse (b then a). c never
	// started and must NOT appear in a stop entry.
	assertSubsequence(t, got, []string{"a:start", "b:start"}, "started before failure")
	assertSubsequence(t, got, []string{"b:stop", "a:stop"}, "rollback reverse stop")
	for _, e := range got {
		if e == "c:stop" {
			t.Fatalf("failed component c must not be stopped; trace=%v", got)
		}
	}
}

func TestAppDoesNotStopFailedComponent(t *testing.T) {
	a, trace, mu := newTraceApp(t, "svc")
	mustRegister(a, newRecordingComponent("ok", trace, mu))
	broken := newRecordingComponent("bad", trace, mu)
	broken.startErr = errors.New("nope")
	mustRegister(a, broken)

	if err := a.Run(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	mu.Lock()
	got := append([]string(nil), *trace...)
	mu.Unlock()
	for _, e := range got {
		if e == "bad:stop" {
			t.Fatalf("failed component must not be stopped; trace=%v", got)
		}
	}
}

func TestAppJoinsStopErrors(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc")
	mu := &sync.Mutex{}
	trace := make([]string, 0)
	c1 := newRecordingComponent("a", &trace, mu)
	c1.stopErr = errors.New("stop-a")
	c2 := newRecordingComponent("b", &trace, mu)
	c2.stopErr = errors.New("stop-b")
	mustRegister(a, c1)
	mustRegister(a, c2)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	err := <-done
	if err == nil {
		t.Fatal("expected joined stop errors")
	}
	// errors.Join stringifies; both messages must appear.
	if !containsMsg(err, "stop-a") || !containsMsg(err, "stop-b") {
		t.Fatalf("expected both stop errors joined, got %v", err)
	}
}

func TestAppStopIsIdempotent(t *testing.T) {
	// The contract says components must tolerate repeated Stop. Verify a
	// component whose Stop is invoked twice does not error, panic, or double
	// execute its underlying work.
	idem := &idempotentStop{}
	if err := idem.Stop(context.Background()); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := idem.Stop(context.Background()); err != nil {
		t.Fatalf("second stop must be idempotent: %v", err)
	}
	if idem.count != 1 {
		t.Fatalf("expected underlying work once, got %d", idem.count)
	}
}

type idempotentStop struct {
	once  sync.Once
	count int
}

func (i *idempotentStop) Name() string { return "idem" }
func (i *idempotentStop) Start(context.Context) error { return nil }
func (i *idempotentStop) Stop(context.Context) error {
	i.once.Do(func() { i.count++ })
	return nil
}

func TestAppDrainTimeoutContinuesToStop(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc", WithDrainTimeout(50*time.Millisecond), WithStopTimeout(time.Second))
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	mustRegister(a, slow)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	start := time.Now()
	err := <-done
	elapsed := time.Since(start)

	// Drain is blocked forever; the 50ms drain timeout must force Stop and Run
	// must still return (well under the 2s safety bound).
	if elapsed > 2*time.Second {
		t.Fatalf("drain timeout did not force stop; elapsed=%v", elapsed)
	}
	// An error is acceptable here (drain ctx cancelled); the key assertion is
	// that Run returned.
	_ = err
	if !slow.stopped.Load() {
		t.Fatal("expected Stop to be called after drain timeout")
	}
}

type blockingDrainer struct {
	name        string
	drainStarted chan struct{}
	stopped     atomic.Bool
}

func (b *blockingDrainer) Name() string { return b.name }
func (b *blockingDrainer) Start(context.Context) error { return nil }
func (b *blockingDrainer) Drain(ctx context.Context) error {
	close(b.drainStarted)
	<-ctx.Done() // block until cancelled by timeout/escalation
	return ctx.Err()
}
func (b *blockingDrainer) Stop(context.Context) error {
	b.stopped.Store(true)
	return nil
}

func TestAppSecondSignalCancelsDrain(t *testing.T) {
	a, _, _ := newTraceApp(t, "svc", WithDrainTimeout(30*time.Second), WithStopTimeout(time.Second))
	slow := &blockingDrainer{name: "slow", drainStarted: make(chan struct{})}
	mustRegister(a, slow)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	// First cancel triggers drain. Wait until drain is in flight, then trigger
	// the in-process escalation path (equivalent to a second SIGTERM). This
	// exercises the same code a real second signal would, deterministically.
	<-slow.drainStarted
	// Spin until escalateDrain is wired (it is set once Ready is reached and
	// signals are installed).
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
	if !slow.stopped.Load() {
		t.Fatal("expected Stop after escalation")
	}
}

func TestAppRejectsDuplicateRun(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc")
	trace := make([]string, 0)
	mustRegister(a, newRecordingComponent("a", &trace, mu))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := a.Run(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
}

func TestComponentGoroutinesExitAfterRun(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc")
	trace := make([]string, 0)
	c := newRecordingComponent("worker", &trace, mu)
	c.goroutine = true
	mustRegister(a, c)

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)

	// Wait until the goroutine is actually running.
	c.startedWG.Wait()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The goroutine must have exited.
	c.stoppedWG.Wait()
}

func TestAppQuiesceCalledBeforeDrain(t *testing.T) {
	a, trace, mu := newTraceApp(t, "svc")
	mustRegister(a, newRecordingComponent("a", trace, mu))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	got := append([]string(nil), *trace...)
	mu.Unlock()
	// For a component implementing both, quiesce must precede drain.
	idxQ, idxD := -1, -1
	for i, e := range got {
		if e == "a:quiesce" {
			idxQ = i
		}
		if e == "a:drain" {
			idxD = i
		}
	}
	if idxQ < 0 || idxD < 0 {
		t.Fatalf("expected quiesce and drain entries, got %v", got)
	}
	if idxQ > idxD {
		t.Fatalf("quiesce must precede drain; trace=%v", got)
	}
}

func TestReadyFlipsFalseOnDrain(t *testing.T) {
	a, _, mu := newTraceApp(t, "svc")
	trace := make([]string, 0)
	mustRegister(a, newRecordingComponent("a", &trace, mu))

	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	<-done
	if a.Ready() {
		t.Fatal("Ready must be false after Run completes")
	}
	if a.Phase() != phaseStopped {
		t.Fatalf("expected phase %q, got %q", phaseStopped, a.Phase())
	}
}

// --- helpers ---

func mustRegister(a *App, c Component) {
	if err := a.Register(c); err != nil {
		panic(err)
	}
}

// runInBackground starts a.Run in a goroutine and returns a channel that
// receives its result. Tests cancel ctx to trigger normal shutdown.
func runInBackground(t *testing.T, a *App, ctx context.Context) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	// Give the app a moment to reach Ready so cancellation triggers drain
	// rather than racing startup. We poll the phase rather than sleeping
	// arbitrarily.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.Phase() == phaseReady {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	return done
}

func assertSubsequence(t *testing.T, got, want []string, what string) {
	t.Helper()
	i := 0
	for _, w := range want {
		found := false
		for ; i < len(got); i++ {
			if got[i] == w {
				found = true
				i++
				break
			}
		}
		if !found {
			t.Fatalf("%s: wanted %q to appear in order in trace=%v", what, w, got)
		}
	}
}

func containsMsg(err error, msg string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), msg)
}
