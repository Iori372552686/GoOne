package runtime

import (
	"context"
	"errors"
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
	defer a.signalCleanup()

	a.markStarting()
	// 在 Run 开始时把观察者一次性附加到 state store。
	for _, o := range a.observers {
		a.state.AddObserver(o)
	}
	if a.onLoadConfig != nil {
		if err := a.onLoadConfig(ctx); err != nil {
			a.markFailed(err)
			return err
		}
	}

	// 阶段 3：按序 Start 组件，失败时回滚。
	started, startErr := a.startComponents(ctx)
	if startErr != nil {
		// 失败组件负责自身的部分清理；App 只 Stop 已成功启动的组件。
		stopErr := a.stopComponents(started, a.stopTimeout)
		a.markFailed(startErr)
		return errors.Join(startErr, stopErr)
	}

	// 阶段 4：Ready。阻塞直到被告知排空。重载会重新进入 Ready。
	src := installSignals()
	defer src.stop()
	a.escalateDrain = src.escalate
	defer func() { a.escalateDrain = nil }()
	if err := a.enterReady(ctx); err != nil {
		// Ready 观察者拒绝了转换：回滚启动。
		stopErr := a.stopComponents(started, a.stopTimeout)
		a.markFailed(err)
		return errors.Join(err, stopErr)
	}
	a.serveReady(ctx, src)

	// 阶段 5：Draining。Quiesce 然后 Drain，受超时约束且可被中断。
	reason := a.currentDrainReason()
	a.enterDraining(ctx, reason)
	drainErr := a.drainComponents(src)

	// 阶段 6：Stopping。无论排空结果如何，Stop 所有已启动组件。
	a.enterStopping(ctx)
	stopErr := a.stopComponents(started, a.stopTimeout)

	a.markStopped()
	if drainErr != nil || stopErr != nil {
		return errors.Join(drainErr, stopErr)
	}
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

// signalCleanup 重置一次性 guard 的可观测面，使被误用的 App 看起来不是半运行状
// 态；guard 本身保持 true，使字面上的第二次 Run 仍报错。（空操作占位，保留以备
// 对称/未来使用。）
func (a *App) signalCleanup() {}

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
func (a *App) serveReady(ctx context.Context, src signalSource) {
	for {
		reason, err := awaitRunReason(ctx, src)
		if reason == "reload" {
			a.handleReload(ctx)
			continue
		}
		// "ctx_done" 或 "terminated"：进入排空。err 是 ctx 错误（terminated 时为
		// nil）；它非致命，只是触发信号。
		_ = err
		a.setDrainReason(reason)
		return
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
func (a *App) drainComponents(src signalSource) error {
	drainStart := time.Now()
	a.mu.Lock()
	started := append([]Component(nil), a.components...)
	a.mu.Unlock()

	drainCtx, drainCancel := context.WithCancel(context.Background())
	defer drainCancel()

	// 升级：第二次终止信号取消排空截止时间。
	go func() {
		select {
		case <-src.secondCh:
			drainCancel()
		case <-drainCtx.Done():
		}
	}()

	// 用配置的超时约束排空。我们用 timer（而非 select 中的 time.After，遵循
	// STYLE），完成后取消。
	timer := time.NewTimer(a.drainTimeout)
	defer timer.Stop()
	go func() {
		select {
		case <-timer.C:
			drainCancel()
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
	// 上报排空耗时与是否超时（drainCtx 被超时取消即视为超时）。
	timedOut := drainCtx.Err() == context.DeadlineExceeded
	observeDrain(time.Since(drainStart).Seconds(), timedOut)
	if timedOut {
		logEventf(eventDrainTimedOut, a.name, "after %.3fs", time.Since(drainStart).Seconds())
	} else {
		logEventf(eventDrainCompleted, a.name, "in %.3fs", time.Since(drainStart).Seconds())
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

// --- 阶段转换（同时驱动遗留 phase 字符串与规范 State 机，使遗留调用方与新 admin
// 端点一致）---

func (a *App) markStarting() {
	a.stateMu.Lock()
	a.alive = true
	a.ready = false
	a.startedAt = time.Now()
	a.phase = phaseStarting
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
	// state store 已起始为 Starting；保持观察者一致。
	logger.Infof("%s starting", a.name)
}

// enterReady 通过状态机把 App 移到 Ready。若 Ready 观察者拒绝转换，返回的 error
// 会中止启动（调用方回滚）。
func (a *App) enterReady(ctx context.Context) error {
	if err := a.state.transition(ctx, StateReady, "all components started", time.Time{}); err != nil {
		return err
	}
	a.stateMu.Lock()
	a.ready = true
	a.phase = phaseReady
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
	logger.Infof("%s ready", a.name)
	return nil
}

func (a *App) enterDraining(ctx context.Context, reason string) {
	deadline := time.Now().Add(a.drainTimeout)
	_ = a.state.transition(ctx, StateDraining, reason, deadline)
	a.stateMu.Lock()
	// readyz 必须在进入 Draining 的瞬间失败，先于任何资源真正排空。
	a.ready = false
	a.phase = phaseDraining
	a.phaseSince = time.Now()
	a.drainReason = reason
	a.stateMu.Unlock()
	logEventf(eventDrainStarted, a.name, "reason=%s", reason)
}

func (a *App) enterStopping(ctx context.Context) {
	_ = a.state.transition(ctx, StateStopping, "stopping", time.Time{})
	a.stateMu.Lock()
	a.ready = false
	a.phase = phaseStopping
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
	logger.Infof("%s stopping", a.name)
}

func (a *App) markFailed(err error) {
	_ = a.state.transition(context.Background(), StateFailed, err.Error(), time.Time{})
	a.stateMu.Lock()
	a.ready = false
	a.alive = false
	a.phase = phaseFailed
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
	logger.Errorf("%s failed | %v", a.name, err)
}

func (a *App) markStopped() {
	_ = a.state.transition(context.Background(), StateStopped, "stopped", time.Time{})
	a.stateMu.Lock()
	a.ready = false
	a.alive = false
	a.phase = phaseStopped
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
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
