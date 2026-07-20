package runtime

import (
	"sync"
	"time"
)

// ComponentReport 是单个组件的外部可观测状态，由 /components 端点返回。它记录时
// 序与最近错误，使卡住或失败的组件无需 grep 日志即可显现。
type ComponentReport struct {
	Name            string `json:"name"`
	State           string `json:"state"`
	Ready           bool   `json:"ready"`
	StartedAt       string `json:"started_at,omitempty"`
	StoppedAt       string `json:"stopped_at,omitempty"`
	StartDurationMs int64  `json:"start_duration_ms,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	Message         string `json:"message,omitempty"`
}

const (
	componentStateStarting = "starting"
	componentStateStarted  = "running"
	componentStateStopping = "stopping"
	componentStateStopped  = "stopped"
	componentStateFailed   = "failed"
)

// componentRecord 是在 tracker.mu 下持有的每组件可变状态。
type componentRecord struct {
	name      string
	state     string
	ready     bool
	startedAt time.Time
	stoppedAt time.Time
	startTook time.Duration
	lastError string
	message   string
}

// ComponentTracker 记录每个已注册组件的生命周期，使 /components 端点能上报
// start/stop 时序与错误。App 在 Start/Drain/Stop 调用前后更新它。
type ComponentTracker struct {
	mu       sync.RWMutex
	records  map[string]*componentRecord
	ordering []string
}

// NewComponentTracker 构建一个空 tracker，用给定组件名（按注册顺序）预填，使端点
// 在组件尚未启动前就能列出全部组件。
func NewComponentTracker(names []string) *ComponentTracker {
	t := &ComponentTracker{records: make(map[string]*componentRecord, len(names))}
	for _, n := range names {
		t.records[n] = &componentRecord{name: n, state: componentStatePending()}
		t.ordering = append(t.ordering, n)
	}
	return t
}

func componentStatePending() string { return "pending" }

// MarkStarting 记录某组件的 Start 已开始。
func (t *ComponentTracker) MarkStarting(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := t.touch(name)
	rec.state = componentStateStarting
	rec.ready = false
	rec.startedAt = time.Now()
	rec.lastError = ""
	rec.message = "starting"
}

// MarkStarted 记录成功的 Start 及其耗时。
func (t *ComponentTracker) MarkStarted(name string, took time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := t.touch(name)
	rec.state = componentStateStarted
	rec.ready = true
	rec.startTook = took
	rec.message = "started"
}

// MarkStartFailed 记录失败的 Start。
func (t *ComponentTracker) MarkStartFailed(name string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := t.touch(name)
	rec.state = componentStateFailed
	rec.ready = false
	rec.lastError = errString(err)
	rec.message = "start failed"
}

// MarkStopping 记录某组件的 Stop 已开始。
func (t *ComponentTracker) MarkStopping(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := t.touch(name)
	rec.state = componentStateStopping
	rec.ready = false
	rec.message = "stopping"
}

// MarkStopped 记录已完成的 Stop。
func (t *ComponentTracker) MarkStopped(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := t.touch(name)
	rec.state = componentStateStopped
	rec.ready = false
	rec.stoppedAt = time.Now()
	rec.message = "stopped"
}

// Report 按注册顺序返回当前组件报告。
func (t *ComponentTracker) Report() []ComponentReport {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]ComponentReport, 0, len(t.ordering))
	for _, name := range t.ordering {
		rec := t.records[name]
		if rec == nil {
			continue
		}
		out = append(out, ComponentReport{
			Name:            rec.name,
			State:           rec.state,
			Ready:           rec.ready,
			StartedAt:       formatTime(rec.startedAt),
			StoppedAt:       formatTime(rec.stoppedAt),
			StartDurationMs: rec.startTook.Milliseconds(),
			LastError:       rec.lastError,
			Message:         rec.message,
		})
	}
	return out
}

// touch 取得或创建 name 的记录。调用方必须持有 t.mu。
func (t *ComponentTracker) touch(name string) *componentRecord {
	rec, ok := t.records[name]
	if !ok {
		rec = &componentRecord{name: name, state: componentStatePending()}
		t.records[name] = rec
		t.ordering = append(t.ordering, name)
	}
	return rec
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
