package runtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State 是 App 的生命周期阶段。它遵循 Agones 风格的演进
// Starting -> Ready -> (Allocated) -> Draining -> Stopping -> Stopped，在不可恢
// 复错误时多数阶段可达 Failed。
type State string

// 规范状态值。它们是 healthz、readyz 与 statez 端点的唯一事实来源。
const (
	// StateStarting：App 正在初始化组件。healthz=200，readyz=503。
	StateStarting State = "starting"
	// StateReady：所有组件已启动；实例接收新流量与新分配。healthz=200，readyz=200。
	StateReady State = "ready"
	// StateAllocated：实例已分配给具体游戏/会话，但仍服务既有流量。
	// healthz=200，readyz=200。
	StateAllocated State = "allocated"
	// StateDraining：不再接收新流量；在途工作正在收尾。healthz=200，readyz=503。
	// 在 SIGTERM 或父 context 取消时到达。
	StateDraining State = "draining"
	// StateStopping：排空完成或超时；组件正被强制停止。healthz=503，readyz=503。
	StateStopping State = "stopping"
	// StateStopped：每个组件都已停止，Run 已返回。healthz=503，readyz=503。
	StateStopped State = "stopped"
	// StateFailed：启动或运行时不可恢复地失败。healthz=503，readyz=503。
	StateFailed State = "failed"
)

// ErrInvalidStateTransition 在转换请求的 (from, to) 不在合法表中时由 transition
// 返回。调用方可用 errors.Is。
var ErrInvalidStateTransition = errors.New("runtime: 非法状态转换")

// legalTransitions 枚举唯一允许的 (from -> to) 迁移。其余一律拒绝，使装配错误立
// 即暴露。
var legalTransitions = map[State]map[State]struct{}{
	StateStarting:  {StateReady: {}, StateFailed: {}, StateStopping: {}},
	StateReady:     {StateAllocated: {}, StateDraining: {}, StateFailed: {}, StateStopping: {}},
	StateAllocated: {StateDraining: {}, StateFailed: {}, StateStopping: {}},
	StateDraining:  {StateStopping: {}, StateFailed: {}},
	StateStopping:  {StateStopped: {}, StateFailed: {}},
	StateStopped:   {},
	StateFailed:    {},
}

// canTransition 上报 from -> to 是否合法。
func canTransition(from, to State) bool {
	targets, ok := legalTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}

// StateChange 描述一次状态转换，传给 StateObserver。
type StateChange struct {
	Previous State     `json:"previous"`
	Current  State     `json:"current"`
	At       time.Time `json:"at"`
	Reason   string    `json:"reason,omitempty"`
	// Deadline 在进入 Draining/Stopping 时的排空/关停截止时间；否则为零。
	Deadline time.Time         `json:"deadline,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// StateObserver 在每次状态转换时被通知。实现必须并发安全且不得阻塞：transition
// 在派发时持有状态锁。
//
// 观察者语义：
//   - 在 Ready/Allocated 时，返回 error 会中止转换并回滚到前一状态（故失败的
//     就绪闸门会阻止服务）。
//   - 在 Draining/Stopping/Stopped 时，错误会被记录但绝不阻断退出。
type StateObserver interface {
	OnStateChange(ctx context.Context, change StateChange) error
}

// ObserverFunc 把函数适配为 StateObserver。
type ObserverFunc func(ctx context.Context, change StateChange) error

// OnStateChange 实现 StateObserver。
func (f ObserverFunc) OnStateChange(ctx context.Context, change StateChange) error {
	if f == nil {
		return nil
	}
	return f(ctx, change)
}

// StateStore 持有当前 State 以及近期的转换上下文。它对并发读者
// （healthz/readyz/statez handler）与单一写者（Run）安全。观察者在锁下派发，故阻
// 塞的观察者表现为卡住的转换，而非被错过的事件。
type StateStore struct {
	mu        sync.RWMutex
	current   State
	since     time.Time
	startedAt time.Time
	reason    string
	deadline  time.Time
	allocated bool
	observers []StateObserver
}

// NewStateStore 构建一个处于 StateStarting 的 store，并在构造时把
// goone_lifecycle_state{state="starting"} 置为 1、其余规范状态置为 0。
//
// 关键不变量（P0-02 修复）：构造即上报，使指标在进程启动瞬间就反映 starting，
// 而非等到第一次状态转换才翻转。
func NewStateStore() *StateStore {
	now := time.Now()
	// 构造时同步上报指标：starting=1，其余状态=0。
	for _, st := range []State{StateStarting, StateReady, StateAllocated, StateDraining, StateStopping, StateStopped, StateFailed} {
		val := 0.0
		if st == StateStarting {
			val = 1.0
		}
		lifecycleStateGauge.WithLabelValues(string(st)).Set(val)
	}
	return &StateStore{
		current:   StateStarting,
		since:     now,
		startedAt: now,
	}
}

// Current 返回当前状态及其进入时间。
func (s *StateStore) Current() (State, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current, s.since
}

// AddObserver 注册一个观察者。必须在 Run 之前调用。
func (s *StateStore) AddObserver(o StateObserver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, o)
}

// transition 移到下一状态并派发观察者。对 Ready/Allocated，观察者 error 会回滚到
// 前一状态并返回该 error。对关停状态，观察者 error 被返回但转换已提交。非法转换
// 返回 ErrInvalidStateTransition 且不派发。
//
// 指标时机（P0-02 修复）：goone_lifecycle_state gauge 的翻转发生在状态**提交成功之
// 后**，而非之前。这样被 Ready/Allocated 观察者拒绝的转换不会留下错误的 gauge 值。
// 关停状态（Draining/Stopping/Stopped/Failed）的转换总是提交，故其 gauge 在派发观察
// 者（best-effort）之前翻转；若观察者返回 error，转换已提交，gauge 保持与新状态一
// 臗。
func (s *StateStore) transition(ctx context.Context, to State, reason string, deadline time.Time) error {
	s.mu.Lock()
	from := s.current
	if !canTransition(from, to) {
		s.mu.Unlock()
		return ErrInvalidStateTransition
	}
	change := StateChange{
		Previous: from,
		Current:  to,
		At:       time.Now(),
		Reason:   reason,
		Deadline: deadline,
	}
	observers := append([]StateObserver(nil), s.observers...)
	// 对关停状态（Draining、Stopping、Stopped、Failed）乐观提交转换，使失败的观察者无法
	// 挂住进程；对 Ready/Allocated 则在前一状态保留，直到观察者同意。
	committingShutdown := to == StateDraining || to == StateStopping || to == StateStopped || to == StateFailed
	if committingShutdown {
		s.applyLocked(to, reason, deadline)
		// 关停转换已提交：翻转 gauge 与状态一致。
		setLifecycleState(from, to)
	}
	s.mu.Unlock()

	var firstErr error
	for _, o := range observers {
		if err := o.OnStateChange(ctx, change); err != nil {
			if !committingShutdown {
				// 回滚：停留于前一状态；gauge 未翻转，保持与 from 一致。
				return err
			}
			// 关停观察者错误是 best-effort：转换已提交，仅上报。
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if !committingShutdown {
		// Ready/Allocated：观察者全部同意后才提交并翻转 gauge。
		s.mu.Lock()
		s.applyLocked(to, reason, deadline)
		s.mu.Unlock()
		setLifecycleState(from, to)
	}
	return firstErr
}

// applyLocked 记录新状态；调用方必须持有 s.mu。
func (s *StateStore) applyLocked(to State, reason string, deadline time.Time) {
	s.current = to
	s.since = time.Now()
	s.reason = reason
	s.deadline = deadline
	if to == StateAllocated {
		s.allocated = true
	}
}

// markAllocated 翻转 allocated 标志而不做完整转换（已在 Ready/Allocated 时由
// Allocate 方法使用）。
func (s *StateStore) markAllocated() {
	s.mu.Lock()
	s.allocated = true
	s.mu.Unlock()
}

// snapshot 返回 statez 端点的外部可观测字段。绝不包含 Metadata 与凭据。
//
// P0-02：startedAt 由 StateStore 自身持有（构造时记录），admin 不再接收 App 的第二
// 份时间，使状态、指标与 admin JSON 始终出自同一事实源。
func (s *StateStore) snapshot(service string) stateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := stateSnapshot{
		Service:   service,
		State:     s.current,
		Since:     formatTime(s.since),
		Reason:    s.reason,
		Allocated: s.allocated,
	}
	if !s.startedAt.IsZero() {
		snap.StartedAt = formatTime(s.startedAt)
	}
	if !s.deadline.IsZero() {
		snap.DrainDeadline = formatTime(s.deadline)
	}
	return snap
}

// healthCode/readyCode 编码状态表中的 healthz/readyz 契约。
func healthCode(s State) int {
	switch s {
	case StateStopping, StateStopped, StateFailed:
		return 503
	default: // Starting、Ready、Allocated、Draining
		return 200
	}
}

func readyCode(s State) int {
	switch s {
	case StateReady, StateAllocated:
		return 200
	default: // Starting、Draining、Stopping、Stopped、Failed
		return 503
	}
}

// stateSnapshot 是 /statez 的 JSON 负载。它刻意省略 config、连接凭据与任何密钥。
type stateSnapshot struct {
	Service       string `json:"service"`
	State         State  `json:"state"`
	StartedAt     string `json:"started_at,omitempty"`
	Since         string `json:"since"`
	Reason        string `json:"reason,omitempty"`
	DrainDeadline string `json:"drain_deadline,omitempty"`
	Allocated     bool   `json:"allocated"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
