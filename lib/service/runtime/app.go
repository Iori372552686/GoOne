package runtime

import (
	"context"
	"errors"
	"fmt"
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
	// ErrDrainEscalated 在第二次终止信号强制取消进行中的排空时作为排空阶段错误的
	// 根因返回。它与"排空时间耗尽"（context.DeadlineExceeded）明确区分：
	// ErrDrainEscalated 表示主动升级，DeadlineExceeded 表示真实超时。二者都不再计
	// 入 drain_timeouts_total（只有 DeadlineExceeded 才是超时）。
	ErrDrainEscalated = errors.New("runtime: 排空被第二次终止信号升级取消")
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

	// 生命周期状态。P0-02 后，phase/ready/alive/drainReason 全部从 StateStore 派生，
	// App 不再维护第二份副本，使 State()、Phase()、Ready()、Alive()、admin JSON 与
	// Prometheus gauge 始终出自同一事实源。
	stateMu     sync.RWMutex
	drainReason string

	// escalateDrain 在非 nil 时，会像收到第二次终止信号一样取消进行中的 Drain。
	// 它在 Run 安装信号后被设置，包级私有，使测试可确定性地触发 escalation 而
	// 无需向测试二进制投递 OS 信号。escalateMu 保护其读写，避免与
	// tryEscalateDrain 的并发读产生 data race。
	escalateDrain func()
	escalateMu    sync.RWMutex

	// state 是驱动 healthz/readyz/statez 的规范生命周期状态机。遗留的 phase 字符
	// 串字段与之保持同步，供仍读取 Phase() 的调用方使用。
	state *StateStore
	// 通过 WithStateObserver 安装的观察者；在 Run 时附加到 state。
	observers []StateObserver

	// tracker 记录每组件 Start/Stop 耗时与错误，供 /components 端点使用。未接线
	// admin server 时可为 nil；start/stop 辅助函数容忍 nil tracker。
	tracker *ComponentTracker
}

// New 用给定服务名与 options 构建 App。名字必须非空；用于日志、指标与 admin 端点。
//
// P0-03：默认创建一个 ComponentTracker（app.tracker），使 /components 端点在任何接线
// 下都能列出 pending/running 组件，无需调用方手动传入。Register 同步把组件名登记为
// pending。可用 WithComponentTracker 覆盖（主要用于测试）。
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
		tracker:      NewComponentTracker(nil),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}

// MustNew 是 New 的便捷封装，在出错时 panic。仅在装配错误即程序员错误的 src/<service>/
// app.go 这类装配代码中使用；库代码应继续用 New 并返回 error。
func MustNew(name string, opts ...Option) *App {
	app, err := New(name, opts...)
	if err != nil {
		panic(err)
	}
	return app
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
	// P0-03：登记组件名为 pending，使 /components 在 Start 前就能列出全部组件。
	if a.tracker != nil {
		a.tracker.MarkPending(name)
	}
	return nil
}

// MustRegister 是在 Register 出错时 panic 的便捷方法。仅在装配错误即程序员错误的装
// 配代码（src/<service>/app.go）中使用。P1-07：支持可变参数，便于
// app.MustRegister(datetime, logger, admin, ...) 一次注册多个组件。
func (a *App) MustRegister(components ...Component) {
	for _, c := range components {
		if err := a.Register(c); err != nil {
			panic(fmt.Errorf("register component %q: %w", componentName(c), err))
		}
	}
}

// componentName 返回组件名，供 MustRegister 的 panic 消息使用。
func componentName(c Component) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name()
}

// Ready 上报 App 是否已完成启动并正在服务。它在 Run 进入 Draining 阶段时立即翻
// 转为 false。
//
// P0-02：从 StateStore 派生（Ready 当且仅当 State 为 Ready 或 Allocated），不再读
// App 自维护的标志，避免与 State()/admin JSON 不一致。
func (a *App) Ready() bool {
	st, _ := a.state.Current()
	return st == StateReady || st == StateAllocated
}

// Alive 上报进程是否仍在运行（尚未到达 Stopped/Failed 终态）。
func (a *App) Alive() bool {
	st, _ := a.state.Current()
	return st != StateStopped && st != StateFailed
}

// Phase 返回当前生命周期阶段名，与 State() 的字符串形式一致。
func (a *App) Phase() string {
	st, _ := a.state.Current()
	return string(st)
}

// State 返回规范的生命周期状态。它是 statez 端点暴露、并被 healthz/readyz 使用
// 的值。
func (a *App) State() State {
	st, _ := a.state.Current()
	return st
}

// Allocate 把实例标记为已分配（分配给某局游戏/会话）。仅当 App 处于 Ready 时执行
// Ready->Allocated 转换；对已 Allocated 的实例幂等返回 nil；其他状态返回
// ErrInvalidStateTransition。Allocate 永不改变就绪性：已分配实例继续服务既有流量。
//
// P0-02：签名从 Allocate() 改为 Allocate(ctx) error，使非法状态转换以明确哨兵错误
// 暴露，而非被静默忽略。
func (a *App) Allocate(ctx context.Context) error {
	current, _ := a.state.Current()
	switch current {
	case StateReady:
		if err := a.state.transition(ctx, StateAllocated, "allocated", time.Time{}); err != nil {
			return err
		}
		return nil
	case StateAllocated:
		// 幂等：已分配，不重复转换。
		return nil
	default:
		return ErrInvalidStateTransition
	}
}

// tryEscalateDrain 返回当前接线的 escalation 触发函数；若 Run 尚未安装信号则返回
// nil。它供测试在确定性时机触发"第二次终止信号"路径，不依赖 OS 信号投递。
func (a *App) tryEscalateDrain() func() {
	a.escalateMu.RLock()
	defer a.escalateMu.RUnlock()
	return a.escalateDrain
}

// setEscalateDrain 在 escalateMu 下安装/清空 escalation 触发函数。
func (a *App) setEscalateDrain(fn func()) {
	a.escalateMu.Lock()
	a.escalateDrain = fn
	a.escalateMu.Unlock()
}

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
