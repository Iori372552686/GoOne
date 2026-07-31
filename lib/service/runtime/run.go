package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// Run 驱动完整的应用生命周期，并阻塞直到关停完成。它返回一个聚合 error（可能为
// nil），合并排空与关停期间观测到的所有失败；启动失败同样会返回。
//
// 序列：
//  1. 拒绝第二次 Run（ErrAlreadyRunning）。
//  2. Starting：可选的 LoadConfig 钩子。
//  3. 按注册顺序 Start 组件；失败则逆序回滚已成功组件并返回 joined error。
//  4. Ready：阻塞在父 context、终止信号或重载信号上。重载在内部处理后重新进入
//     Ready。
//  5. Draining：按逆序 Quiesce 然后 Drain 组件，受排空超时约束；第二次终止信号
//     会提前取消 Drain。
//  6. Stopping：按逆序 Stop 所有组件，受关停超时约束；某个 Stop 失败绝不跳过其
//     余组件。
//  7. Stopped。
//
// Run 不得与自身并发调用，或调用超过一次。
func (a *App) Run(ctx context.Context) error {
	if err := a.beginRun(); err != nil {
		return err
	}

	a.markStarting()
	// 在 Run 开始时把观察者一次性附加到 state store。
	for _, o := range a.observers {
		a.state.AddObserver(o)
	}

	// 阶段 2：安装信号源。提前到 onLoadConfig/Start 之前，使启动阶段收到 SIGTERM/
	// SIGINT 能立即取消进行中的 Component.Start。signalCtx 由第一个终止
	// 信号取消，传给 onLoadConfig 与 startComponents；startupDone 用于在启动成功后把
	// termCh 的消费权从启动监督 goroutine 交接给 serveReady 内部的 awaitRunReason。
	src := installSignals()
	defer src.stop()
	a.setEscalateDrain(src.escalate)
	defer a.setEscalateDrain(nil)
	a.setInjectStartupSignal(src.injectForTest)
	defer a.setInjectStartupSignal(nil)

	signalCtx, signalCancel := context.WithCancelCause(ctx)
	defer signalCancel(context.Canceled)
	startupDone := make(chan struct{})
	go func() {
		select {
		case <-src.termCh:
			signalCancel(ErrStartupInterrupted)
		case <-startupDone:
			// 启动完成，把 termCh 消费权交给 serveReady。
		}
	}()

	if a.onLoadConfig != nil {
		if err := a.onLoadConfig(signalCtx); err != nil {
			close(startupDone)
			a.markFailed(err)
			return err
		}
	}

	// 阶段 3：按序 Start 组件，失败时回滚。用 signalCtx 使启动期信号能取消 Start。
	started, startErr := a.startComponents(signalCtx)
	close(startupDone) // 启动阶段结束，释放启动监督 goroutine。
	if startErr != nil {
		// 失败组件负责自身的部分清理；App 只 STOP 已成功启动的组件。
		stopErr := a.stopComponents(started, a.stopTimeout)
		a.markFailed(startErr)
		return errors.Join(startErr, stopErr)
	}

	// 启动 cause 检查。只有全部 Start 成功且启动期终止信号未到达（signalCtx 仍
	// 未被取消）时才允许进入 Ready。不遵守 context、在取消后仍返回 nil 的 Start 不得让
	// App 进入 Ready。
	if cause := context.Cause(signalCtx); cause != nil {
		stopErr := a.stopComponents(started, a.stopTimeout)
		interruptErr := cause
		if errors.Is(interruptErr, context.Canceled) {
			interruptErr = ErrStartupInterrupted
		}
		a.markFailed(interruptErr)
		return errors.Join(interruptErr, stopErr)
	}

	// 阶段 4：Ready。阻塞直到被告知排空。重载会重新进入 Ready。
	if err := a.enterReady(ctx); err != nil {
		// Ready 观察者拒绝了转换：回滚启动。
		stopErr := a.stopComponents(started, a.stopTimeout)
		a.markFailed(err)
		return errors.Join(err, stopErr)
	}
	// 监督实现 RuntimeErrorSource 的组件（HTTP/gRPC listener 等）。首个非 nil
	// 运行期错误触发标准 Quiesce/Drain/Stop 关停，终态 Failed。watcher 返回的错误
	// 会作为排空原因并最终使 Run 返回带组件名的 error。
	runtimeErr := a.serveReady(ctx, src)

	// 阶段 5：Draining。Quiesce 然后 Drain，受超时约束且可被中断。
	reason := a.currentDrainReason()
	a.enterDraining(ctx, reason)
	drainErr := a.drainComponents(src)
	// 运行期错误并入排空错误链，使 Run 返回值包含组件级根因。
	if runtimeErr != nil {
		drainErr = errors.Join(drainErr, runtimeErr)
	}

	// 阶段 6：Stopping。无论排空结果如何，Stop 所有已启动组件。
	a.enterStopping(ctx)
	stopErr := a.stopComponents(started, a.stopTimeout)

	// 终态（修复）：runtimeErr、drainErr、stopErr 任一非 nil 都走 Failed，且每个
	// 终态错误都包含组件名与阶段名（由 errors.Join 保持 %w/Unwrap 链）。
	// 关键不变量：不得先提交 Stopped 再改成 Failed。
	terminalErr := errors.Join(runtimeErr, drainErr, stopErr)
	if terminalErr != nil {
		a.markFailed(terminalErr)
		return terminalErr
	}
	a.markStopped()
	return nil
}

// beginRun 占用一次性的 Run 名额。它因持有 a.mu 而对并发调用方安全。
func (a *App) beginRun() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runStarted {
		return ErrAlreadyRunning
	}
	a.runStarted = true
	return nil
}

// startComponents 按序 Start 每个已注册组件。它返回已成功启动的组件切片（按
// Start 顺序）与遇到的第一个 error（若有）。出错时，失败组件**不**包含在返回切
// 片中，也**不**被 Stop。
func (a *App) startComponents(ctx context.Context) ([]Component, error) {
	a.mu.Lock()
	components := append([]Component(nil), a.components...)
	a.mu.Unlock()

	started := make([]Component, 0, len(components))
	for _, c := range components {
		name := c.Name()
		logEvent(eventComponentStarting, a.name, "component "+name)
		if a.tracker != nil {
			a.tracker.MarkStarting(name)
		}
		begin := time.Now()
		if err := c.Start(ctx); err != nil {
			logger.Errorf("%s component %q start failed | %v", a.name, name, err)
			logEventError(eventComponentStartFailed, a.name+"."+name, err)
			if a.tracker != nil {
				a.tracker.MarkStartFailed(name, err)
			}
			observeComponentStart(name, time.Since(begin).Seconds())
			incComponentStartFailure(name)
			return started, err
		}
		observeComponentStart(name, time.Since(begin).Seconds())
		if a.tracker != nil {
			a.tracker.MarkStarted(name, time.Since(begin))
		}
		started = append(started, c)
		logEventf(eventComponentStarted, a.name, "component %s (%.3fs)", name, time.Since(begin).Seconds())
	}
	return started, nil
}

// serveReady 在 Ready 阶段阻塞。当父 context 取消、终止信号到达、或（处理后）
// 重载完成时返回。重载在内部处理，循环重新进入 Ready。
// serveReady 在 Ready 阶段阻塞。当父 context 取消、终止信号到达、重载（处理后循
// 环）或运行期错误到达时返回。
//
// 返回值 runtimeErr 非 nil 表示由 RuntimeErrorSource 监督触发的关停（如
// HTTP/gRPC listener 意外死亡）；调用方据此走 Failed 终态。nil 表示正常信号/ctx 关
// 停，走 Stopped 终态。
func (a *App) serveReady(ctx context.Context, src signalSource) error {
	// 收集实现 RuntimeErrorSource 的已注册组件，启动一个汇聚 watcher。
	a.mu.Lock()
	components := append([]Component(nil), a.components...)
	a.mu.Unlock()
	runtimeErrCh := make(chan error, 1)
	watcherCtx, watcherCancel := context.WithCancel(ctx)
	defer watcherCancel()
	// 每个 RuntimeErrorSource 启动一个汇聚 watcher；收到非 nil error 时用组件名包装，
	// 使 Run 返回的 error 含组件身份（component.go 的契约要求）。
	for _, c := range components {
		rs, ok := c.(RuntimeErrorSource)
		if !ok {
			continue
		}
		name := c.Name()
		go func(rs RuntimeErrorSource, name string) {
			select {
			case err, ok := <-rs.RuntimeErrors():
				if !ok || err == nil {
					return
				}
				wrapped := fmt.Errorf("runtime: component %q runtime error: %w", name, err)
				select {
				case runtimeErrCh <- wrapped:
				default:
				}
			case <-watcherCtx.Done():
			}
		}(rs, name)
	}

	// 用一个 reason channel 统一信号路径与运行期错误路径，避免 awaitRunReason 阻塞时
	// 错过运行期错误。
	reasonCh := make(chan string, 1)
	go func() {
		reason, _ := awaitRunReason(ctx, src)
		select {
		case reasonCh <- reason:
		default:
		}
	}()

	for {
		select {
		case err := <-runtimeErrCh:
			// 运行期错误：触发排空，终态 Failed。
			a.setDrainReason("runtime_error")
			return err
		case reason := <-reasonCh:
			if reason == "reload" {
				a.handleReload(ctx)
				// 重新进入等待：启动新的 awaitRunReason。
				go func() {
					r, _ := awaitRunReason(ctx, src)
					select {
					case reasonCh <- r:
					default:
					}
				}()
				continue
			}
			a.setDrainReason(reason)
			return nil
		}
	}
}

// handleReload 调用可选的重载钩子。错误会被记录但绝不中止进程，遵循重载契约。
func (a *App) handleReload(ctx context.Context) {
	if a.onReload == nil {
		return
	}
	reloadCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := a.onReload(reloadCtx); err != nil {
		logger.Errorf("%s reload failed | %v", a.name, err)
		return
	}
	logger.Infof("%s reload complete", a.name)
}

// drainComponents 按逆序对所有已启动组件执行 Quiesce 然后 Drain。整个阶段受排空
// 超时约束；第二次终止信号（src.secondCh 关闭）会立即取消它。返回所有
// Quiesce/Drain 失败的 joined error。
//
// 取消原因（修复）：
//   - 排空时间耗尽：drainCtx 的根因是 context.DeadlineExceeded；计入
//     drain_timeouts_total。
//   - 第二次终止信号升级：drainCtx 的根因是 ErrDrainEscalated；不计入超时。
//
// 关键不变量：drainCtx 由 context.WithTimeout 派生，使 drainCtx.Err() 在超时时真
// 正返回 DeadlineExceeded（而非 Canceled）。升级与超时通过 context.WithCancelCause
// 的 cause 区分。历史实现用 WithCancel + goroutine 调 drainCancel 模拟超时，导致
// drainCtx.Err() 永远返回 Canceled、timedOut 判定永远为 false、
// drain_timeouts_total 永不增长。
func (a *App) drainComponents(src signalSource) error {
	drainStart := time.Now()
	a.mu.Lock()
	started := append([]Component(nil), a.components...)
	a.mu.Unlock()

	// 内层 cancel-cause 承载"升级"根因；外层 WithTimeout 承载"时间耗尽"根因。
	// 二者任一触发都会让 drainCtx.Done() 关闭，但 cause 不同：
	//   - 升级：cancelCause(ErrDrainEscalated)
	//   - 超时：context.WithTimeout 自身的 DeadlineExceeded
	escalateCtx, cancelEscalate := context.WithCancelCause(context.Background())
	defer cancelEscalate(context.Canceled)

	drainCtx, cancelTimeout := context.WithTimeout(escalateCtx, a.drainTimeout)
	defer cancelTimeout()

	// 升级：第二次终止信号携带 ErrDrainEscalated 根因取消排空。
	go func() {
		select {
		case <-src.secondCh:
			cancelEscalate(ErrDrainEscalated)
		case <-drainCtx.Done():
		}
	}()

	var errs []error
	for i := len(started) - 1; i >= 0; i-- {
		c := started[i]
		if q := quiescerOf(c); q != nil {
			if err := q.Quiesce(drainCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Errorf("%s component %q quiesce failed | %v", a.name, c.Name(), err)
				errs = append(errs, err)
			}
		}
	}
	for i := len(started) - 1; i >= 0; i-- {
		c := started[i]
		if d := drainerOf(c); d != nil {
			if err := d.Drain(drainCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Errorf("%s component %q drain failed | %v", a.name, c.Name(), err)
				errs = append(errs, err)
			}
		}
	}

	// 判定取消根因：只有 DeadlineExceeded 才算超时；ErrDrainEscalated 是升级。
	cause := context.Cause(drainCtx)
	timedOut := errors.Is(cause, context.DeadlineExceeded)
	escalated := errors.Is(cause, ErrDrainEscalated)
	observeDrain(time.Since(drainStart).Seconds(), timedOut)
	switch {
	case timedOut:
		logEventf(eventDrainTimedOut, a.name, "after %.3fs", time.Since(drainStart).Seconds())
	case escalated:
		logEventf(eventDrainEscalated, a.name, "after %.3fs", time.Since(drainStart).Seconds())
	default:
		logEventf(eventDrainCompleted, a.name, "in %.3fs", time.Since(drainStart).Seconds())
	}

	// 把取消根因也并入返回的 error 链，使调用方可用 errors.Is 区分超时/升级。
	if cause != nil && !errors.Is(cause, context.Canceled) {
		errs = append(errs, cause)
	}
	return errors.Join(errs...)
}

// stopComponents 按逆序 Stop 给定组件，受关停超时约束。某组件的 Stop 错误会被记
// 录并收集，但绝不阻止停止其余组件。返回 joined error。
func (a *App) stopComponents(started []Component, timeout time.Duration) error {
	stopCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var errs []error
	for i := len(started) - 1; i >= 0; i-- {
		c := started[i]
		name := c.Name()
		if a.tracker != nil {
			a.tracker.MarkStopping(name)
		}
		if err := c.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Errorf("%s component %q stop failed | %v", a.name, name, err)
			logEventError(eventComponentStopFailed, a.name+"."+name, err)
			errs = append(errs, err)
		}
		if a.tracker != nil {
			a.tracker.MarkStopped(name)
		}
	}
	return errors.Join(errs...)
}

// --- 阶段转换（状态机为唯一事实源，不再维护重复的 phase/ready/alive 副本）---

func (a *App) markStarting() {
	// state store 已在 NewStateStore 构造时起始为 Starting 且上报了 gauge。
	logger.Infof("%s starting", a.name)
}

// enterReady 通过状态机把 App 移到 Ready。若 Ready 观察者拒绝转换，返回的 error
// 会中止启动（调用方回滚）。
func (a *App) enterReady(ctx context.Context) error {
	if err := a.state.transition(ctx, StateReady, "all components started", time.Time{}); err != nil {
		return err
	}
	logger.Infof("%s ready", a.name)
	return nil
}

func (a *App) enterDraining(ctx context.Context, reason string) {
	deadline := time.Now().Add(a.drainTimeout)
	_ = a.state.transition(ctx, StateDraining, reason, deadline)
	a.setDrainReason(reason)
	logEventf(eventDrainStarted, a.name, "reason=%s", reason)
}

func (a *App) enterStopping(ctx context.Context) {
	_ = a.state.transition(ctx, StateStopping, "stopping", time.Time{})
	logger.Infof("%s stopping", a.name)
}

func (a *App) markFailed(err error) {
	_ = a.state.transition(context.Background(), StateFailed, err.Error(), time.Time{})
	logger.Errorf("%s failed | %v", a.name, err)
}

func (a *App) markStopped() {
	_ = a.state.transition(context.Background(), StateStopped, "stopped", time.Time{})
	logger.Infof("%s stopped", a.name)
	logger.Flush()
}

func (a *App) setDrainReason(reason string) {
	a.stateMu.Lock()
	a.drainReason = reason
	a.stateMu.Unlock()
}

func (a *App) currentDrainReason() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.drainReason
}
