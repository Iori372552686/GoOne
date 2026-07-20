package runtime

import (
	"context"
	"errors"
	"sync"
	"time"
)

// 哨兵错误，由 App 返回。调用方可用 errors.Is 区分；它们不会被不透明包装。
var (
	// ErrAlreadyRunning 在同一 App 上多次调用 Run 时返回。App 按设计为一次性。
	ErrAlreadyRunning = errors.New("runtime: app 已在运行")
	// ErrEmptyName 在 New 时服务名为空时返回。
	ErrEmptyName = errors.New("runtime: 服务名不能为空")
	// ErrDuplicateComponent 在 Register 时同名组件已注册时返回。
	ErrDuplicateComponent = errors.New("runtime: 重复的组件名")
	// ErrNilComponent 在 Register 时传入 nil 组件时返回。
	ErrNilComponent = errors.New("runtime: nil 组件")
	// ErrEmptyComponentName 在 Register 时组件返回空名字时返回。
	ErrEmptyComponentName = errors.New("runtime: 组件名不能为空")
)

// 默认超时。它们约束排空与关停阶段，使行为异常的组件无法无限期挂住进程。服务
// 可通过 WithDrainTimeout / WithStopTimeout 按 App 覆盖。
const (
	DefaultDrainTimeout = 30 * time.Second
	DefaultStopTimeout  = 10 * time.Second
)

// Option 在构造期配置 App。
type Option func(*App)

// WithDrainTimeout 设置 App.Run 在 Drain 阶段（跨所有组件）的最大总耗时。超时的
// 排空会被取消并进入 Stop。必须为正；零值回退到 DefaultDrainTimeout。
func WithDrainTimeout(d time.Duration) Option {
	return func(a *App) {
		if d > 0 {
			a.drainTimeout = d
		}
	}
}

// WithStopTimeout 设置 App.Run 在 Stop 阶段（跨所有组件）的最大总耗时。必须为
// 正；零值回退到 DefaultStopTimeout。
func WithStopTimeout(d time.Duration) Option {
	return func(a *App) {
		if d > 0 {
			a.stopTimeout = d
		}
	}
}

// WithReload 安装一个在收到平台重载信号（Unix 上为 SIGUSR1；Windows 永远不会）
// 时被调用的回调。回调不得修改运行时资源；它接收一个可安全重载的快照，返回的
// error（若非 nil）会被记录但绝不中止进程。可为 nil。
func WithReload(fn func(ctx context.Context) error) Option {
	return func(a *App) {
		a.onReload = fn
	}
}

// WithLoadConfig 安装一个在任何组件 Start 之前、Run 最开始被调用一次的回调。这
// 是发布初始不可变配置快照的位置。返回 error 会中止启动。可为 nil。
func WithLoadConfig(fn func(ctx context.Context) error) Option {
	return func(a *App) {
		a.onLoadConfig = fn
	}
}

// WithStateObserver 注册一个在每次状态转换时被通知的 StateObserver。Ready/
// Allocated 的回滚语义见 StateStore。必须在 Run 之前调用。
func WithStateObserver(o StateObserver) Option {
	return func(a *App) {
		a.observers = append(a.observers, o)
	}
}

// WithComponentTracker 安装一个记录每组件 Start/Stop 耗时与错误的 tracker，通过
// /components admin 端点暴露。设置后，Run 会在每次 Start/Stop 前后更新它。
func WithComponentTracker(t *ComponentTracker) Option {
	return func(a *App) {
		a.tracker = t
	}
}

// App 持有一组 Component，并由单次 Run 驱动其生命周期。
//
// App 为一次性：Run 最多调用一次。组件在 Run 前用 Register 注册；注册顺序定义
// Start 顺序，逆序用于 Quiesce、Drain 与 Stop。
//
// App 可安全地构造与 Register，但 Run 不可并发调用（且第二次调用会以
// ErrAlreadyRunning 拒绝）。
type App struct {
	name string

	mu         sync.Mutex
	components []Component
	names      map[string]struct{}
	runStarted bool

	// 排空与关停阶段的超时预算。
	drainTimeout time.Duration
	stopTimeout  time.Duration

	// 可选钩子。
	onLoadConfig func(ctx context.Context) error
	onReload     func(ctx context.Context) error

	// 生命周期状态。由 stateMu 保护。完整的 State 枚举、合法转换与观察者在状态
	// 机任务中引入；这里只追踪一个阶段字符串加上外部可观测的 ready/alive 标志，
	// 供调用方与测试依赖。
	stateMu     sync.RWMutex
	phase       string
	phaseSince  time.Time
	ready       bool
	alive       bool
	startedAt   time.Time
	drainReason string

	// escalateDrain 在非 nil 时，会像收到第二次终止信号一样取消进行中的 Drain。
	// 它在 Run 安装信号后被设置，包级私有，使测试可确定性地触发 escalation 而
	// 无需向测试二进制投递 OS 信号。
	escalateDrain func()

	// state 是驱动 healthz/readyz/statez 的规范生命周期状态机。遗留的 phase 字符
	// 串字段与之保持同步，供仍读取 Phase() 的调用方使用。
	state *StateStore
	// 通过 WithStateObserver 安装的观察者；在 Run 时附加到 state。
	observers []StateObserver

	// tracker 记录每组件 Start/Stop 耗时与错误，供 /components 端点使用。未接线
	// admin server 时可为 nil；start/stop 辅助函数容忍 nil tracker。
	tracker *ComponentTracker
}

// startedAtLocked 返回进程启动时间。调用方可持有 stateMu 也可不持有；它为安全
// 起见在 stateMu 下读取该近似原子的字段。
func (a *App) startedAtLocked() time.Time {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.startedAt
}

// New 用给定服务名与 options 构建 App。名字必须非空；用于日志、指标与 admin 端点。
func New(name string, opts ...Option) (*App, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	a := &App{
		name:         name,
		names:        make(map[string]struct{}),
		drainTimeout: DefaultDrainTimeout,
		stopTimeout:  DefaultStopTimeout,
		state:        NewStateStore(),
	}
	a.setPhase(phaseCreated)
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}

// Name 返回 New 时传入的服务名。
func (a *App) Name() string { return a.name }

// Register 向 App 添加一个组件。必须在 Run 之前调用。重复名字、nil 组件与空名字
// 会被立即拒绝，使装配错误在启动期而非运行期暴露。
//
// Register 可在装配期由单 goroutine 调用；不用于并发场景。
func (a *App) Register(c Component) error {
	if c == nil {
		return ErrNilComponent
	}
	name := c.Name()
	if name == "" {
		return ErrEmptyComponentName
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runStarted {
		return ErrAlreadyRunning
	}
	if _, exists := a.names[name]; exists {
		return ErrDuplicateComponent
	}
	a.names[name] = struct{}{}
	a.components = append(a.components, c)
	return nil
}

// MustRegister 是在 Register 出错时 panic 的便捷方法。仅在装配错误即程序员错误
// 的装配代码中使用。
func (a *App) MustRegister(c Component) {
	if err := a.Register(c); err != nil {
		panic(err)
	}
}

// Ready 上报 App 是否已完成启动并正在服务。它在 Run 进入 Draining 阶段时立即翻
// 转为 false。
func (a *App) Ready() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.ready
}

// Alive 上报进程是否仍在运行（尚未完成 Stop）。
func (a *App) Alive() bool {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.alive
}

// Phase 返回当前生命周期阶段名。阶段集合在 P0-01 生命周期内稳定；更丰富的 State
// 机在状态机任务中到来。
func (a *App) Phase() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.phase
}

// State 返回规范的生命周期状态。它是 statez 端点暴露、并被 healthz/readyz 使用
// 的值。
func (a *App) State() State {
	st, _ := a.state.Current()
	return st
}

// Allocate 把实例标记为已分配（分配给某局游戏/会话）。仅当 App 处于 Ready 或已
// Allocated 时有效；其他状态忽略。Allocate 永不改变就绪性：已分配实例继续服务
// 既有流量。
func (a *App) Allocate() {
	current, _ := a.state.Current()
	if current == StateReady || current == StateAllocated {
		// 适用时执行 Ready->Allocated 转换。
		if current == StateReady {
			_ = a.state.transition(context.Background(), StateAllocated, "allocated", time.Time{})
		} else {
			a.state.markAllocated()
		}
	}
}

// setPhase 在 stateMu 下更新阶段。调用方必须未持有 stateMu 锁。
func (a *App) setPhase(phase string) {
	a.stateMu.Lock()
	a.phase = phase
	a.phaseSince = time.Now()
	a.stateMu.Unlock()
}

// 最小阶段常量。导出的 State 类型与转换表位于状态机任务；这些目前为包级私有。
const (
	phaseCreated  = "created"
	phaseStarting = "starting"
	phaseReady    = "ready"
	phaseDraining = "draining"
	phaseStopping = "stopping"
	phaseStopped  = "stopped"
	phaseFailed   = "failed"
)

// NewFromModules 通过把 modules 装配进一个全新 Registry 来构建 App。它是为多模块
// 服务推荐的构造器。Registry 立即 Seal；Run 将按序 Start 其组件。
//
// 模块注册失败时返回错误，且不构建 App。
func NewFromModules(name string, modules []Module, opts ...Option) (*App, error) {
	a, err := New(name, opts...)
	if err != nil {
		return nil, err
	}
	reg := NewRegistry()
	for _, m := range modules {
		if err := reg.RegisterModule(m); err != nil {
			return nil, err
		}
	}
	components, err := reg.Seal()
	if err != nil {
		return nil, err
	}
	for _, c := range components {
		if err := a.Register(c); err != nil {
			return nil, err
		}
	}
	return a, nil
}
