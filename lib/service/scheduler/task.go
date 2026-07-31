// Package scheduler 提供受生命周期管理的周期任务（Task），取代旧的
// application.Run 全局 10ms Tick。
//
// 设计要点：
//   - 每个 Task 有固定周期 Interval、可选 RunOnStart、默认禁止重入（NonOverlap）。
//   - 使用可停止的 timer/ticker，不用 time.After 循环。
//   - Stop 取消 context 并等待当前 Run 返回（不创建叠加 goroutine）。
//   - 上一次未完成时跳过本次并增加 skipped 计数，不堆积。
//   - panic 经 safego 风格 recover 记录并上报，绝不调 Fatal。
//
// Task 作为 runtime.Component 注册到 App：Start 启动 ticker goroutine，Stop 取消并
// join。这样旧的"OnTick 每 10ms 唤醒"被精确周期的 Task 取代，空闲服务不再轮询。
package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// Task 是一个受生命周期管理的周期任务。
//
// 把它注册为 runtime.Component（Name 返回 TaskName）即可获得 Start/Stop 语义；
// Start 启动周期 goroutine，Stop 取消并等待在途 Run 退出。NonOverlap=true（默认）
// 时，若上一次 Run 未完成，本次触发被跳过并计入 skipped 计数，不创建叠加 goroutine。
//
// Inline=true 时绕过 goroutine 池：trigger 直接在 loop 内同步调用 Run，不起额外
// goroutine、不走 TryLock、不记 skipped。专用于极轻量高频任务（如 datetime.Tick，
// 纳秒级单字段赋值），避免每周期 goroutine 创建/调度开销带来的抖动与浪费。
// 使用前提：Run 必须非阻塞且绝不会重叠（否则会拖慢整个 loop 的周期节拍）。
type Task struct {
	TaskName   string
	Interval   time.Duration
	RunOnStart bool
	NonOverlap bool
	// Inline 为 true 时在 loop 内同步执行 Run。见类型注释。默认 false。
	Inline bool
	Run    func(ctx context.Context) error

	// 运行时状态。
	stopCh chan struct{}
	// inFlight 守护所有正在执行的 trigger goroutine；inFlightDone 在 loop 退出且在途
	// 全部完成后关闭，供 Stop join。
	inFlight     sync.WaitGroup
	inFlightDone chan struct{}
	runMu        sync.Mutex
	running      atomic.Bool

	started atomic.Bool
	skipped atomic.Int64

	cancel context.CancelFunc
}

// New 构建一个 Task。name 为组件名；interval 为周期；run 为任务体。默认 NonOverlap
// 为 true、RunOnStart 为 false。
func New(name string, interval time.Duration, run func(ctx context.Context) error) *Task {
	return &Task{
		TaskName:   name,
		Interval:   interval,
		NonOverlap: true,
		Run:        run,
	}
}

// Name 实现 runtime.Component。
func (t *Task) Name() string { return t.TaskName }

// Start 实现 runtime.Component。启动周期 goroutine。若 Interval<=0 且无 RunOnStart，
// 视为一次性无操作组件（直接返回 nil），便于条件装配。
func (t *Task) Start(ctx context.Context) error {
	if t == nil || t.Run == nil {
		return nil
	}
	if !t.started.CompareAndSwap(false, true) {
		return nil // 幂等：已启动。
	}

	if t.Interval <= 0 && !t.RunOnStart {
		// 无周期且不立即跑：空任务，不启 goroutine。
		return nil
	}

	t.stopCh = make(chan struct{})
	t.inFlightDone = make(chan struct{})
	runCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	// RunOnStart 与周期循环都交给 loop 统一托管，使 Stop 通过 inFlightDone 能 join 所有
	// 在途 execute（loop 内串行调用 execute，不会重叠）。
	go t.loop(runCtx)
	return nil
}

// Stop 实现 runtime.Component。取消 context 并等待在途 Run 退出。幂等。
func (t *Task) Stop(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if !t.started.Load() {
		return nil
	}
	if t.cancel != nil {
		t.cancel()
	}
	if t.stopCh != nil {
		// 通知 loop 退出（loop 也会因 ctx 取消退出，这里双保险）。
		select {
		case <-t.stopCh:
		default:
			close(t.stopCh)
		}
	}
	// 等待 loop 退出且所有在途 execute 完成，或受 ctx 约束。
	if t.inFlightDone != nil {
		select {
		case <-t.inFlightDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Skipped 返回因 NonOverlap 而被跳过的触发次数（用于指标/诊断）。
func (t *Task) Skipped() int64 {
	if t == nil {
		return 0
	}
	return t.skipped.Load()
}

// loop 是周期循环主体。它只负责"到点触发"——把每次执行交给独立 goroutine（trigger），
// 这样 loop 不会被一次慢 Run 阻塞，NonOverlap 才能真正在上一次未完成时跳过本次。
// 使用 time.Timer（复用，遵循 STYLE 禁用 time.After）。
//
// 若 RunOnStart 为真，先立即触发一次；若 Interval<=0（仅 RunOnStart 一次性），触发
// 一次后即返回。
func (t *Task) loop(ctx context.Context) {
	// RunOnStart：立即触发一次。
	if t.RunOnStart {
		t.trigger(ctx)
	}
	// 仅一次性：触发后等待在途完成并关闭 inFlightDone（供 Stop join）。
	if t.Interval <= 0 {
		t.waitInFlightAndClose()
		return
	}

	timer := time.NewTimer(t.Interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			t.waitInFlightAndClose()
			return
		case <-t.stopCh:
			t.waitInFlightAndClose()
			return
		case <-timer.C:
			t.trigger(ctx)
			// 复用 timer：Stop+drain+Reset。
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(t.Interval)
		}
	}
}

// trigger 决定是否启动一次执行。
//   - Inline：直接在 loop 内同步调用 runOnce，零 goroutine 开销。要求 Run 极轻量
//     且非阻塞（如 datetime.Tick），否则会拖慢 loop 节拍。
//   - NonOverlap：TryLock 失败则跳过并计 skipped；成功则在独立 goroutine 内执行，
//     loop 不被慢 Run 阻塞。
//   - 其它：每次都在独立 goroutine 内执行（允许叠加），仍受 inFlight 守护。
func (t *Task) trigger(ctx context.Context) {
	if t.Inline {
		// 同步执行：loop 自身串行触发，天然不重叠，无需 TryLock / inFlight 计数。
		// 仍走 runOnce 以获得 panic recover 与耗时上报。
		t.runOnce(ctx)
		return
	}
	if t.NonOverlap {
		if !t.runMu.TryLock() {
			t.skipped.Add(1)
			incTaskSkipped(t.TaskName)
			return
		}
		// 拿到锁：在 goroutine 内执行，完成后释放。loop 可立即处理下一周期。
		t.inFlight.Add(1)
		go func() {
			defer t.inFlight.Done()
			defer t.runMu.Unlock()
			t.runOnce(ctx)
		}()
		return
	}
	// 非 NonOverlap：允许叠加，每次都启动 goroutine（仍受 inFlight 计数守护）。
	t.inFlight.Add(1)
	go func() {
		defer t.inFlight.Done()
		t.runOnce(ctx)
	}()
}

// runOnce 执行一次 Run，带 panic recover 与耗时上报。调用方负责 NonOverlap 锁与
// inFlight 计数。
func (t *Task) runOnce(ctx context.Context) {
	runStart := time.Now()
	t.running.Store(true)
	defer t.running.Store(false)
	defer func() {
		if r := recover(); r != nil {
			// 不调 Fatal：记录并继续，使后续周期仍可运行。
			logger.Errorf("scheduler task %q panic recovered | %v", t.TaskName, r)
		}
	}()
	if err := t.Run(ctx); err != nil && !isCtxErr(err) {
		logger.Errorf("scheduler task %q run error | %v", t.TaskName, err)
	}
	observeTaskRun(t.TaskName, time.Since(runStart).Seconds())
}

// waitInFlightAndClose 等待所有在途 execute 完成后关闭 inFlightDone，供 Stop join。
func (t *Task) waitInFlightAndClose() {
	t.inFlight.Wait()
	close(t.inFlightDone)
}

func isCtxErr(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
