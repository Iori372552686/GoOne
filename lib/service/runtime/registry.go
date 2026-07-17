package runtime

import (
	"errors"
	"sync"
)

// Module 描述一个装配单元。其唯一职责是在装配阶段把组件、SSRPC binding 与 driver
// 注册到 Registry。Module 不得在 Register 中连接 Redis/Bus/DB 或启动 goroutine——
// 那些工作属于它注册的 Component，稍后由 Start 执行。
type Module interface {
	// Name 在 App 内唯一，用于错误消息。
	Name() string
	// Register 把本模块的组件与 binding 声明到 r。除注册表变更外必须无副作用；特别
	// 是不得执行 I/O 或启动 goroutine。返回 error 会中止装配：不启动任何组件。
	Register(r *Registry) error
}

// ModuleFunc 把函数适配为 Module。名字固定；函数执行注册。
type ModuleFunc struct {
	ModuleName string
	OnRegister func(r *Registry) error
}

// Name 实现 Module。
func (m ModuleFunc) Name() string { return m.ModuleName }

// Register 实现 Module。
func (m ModuleFunc) Register(r *Registry) error {
	if m.OnRegister == nil {
		return nil
	}
	return m.OnRegister(r)
}

// Registry 是组件的装配期容器。Module 通过 Register 填充它；App.Seal 把它冻结为
// 一个不可变、有序的组件列表，驱动 Start/Quiesce/Drain/Stop。
//
// Registry 在本层刻意只追踪 Component。SSRPC 与 Driver 注册表是独立的（它们有各自
// 的 binding/driver 语义），在各自的包内接线；此处的 Registry 是 Start 顺序的唯一
// 事实来源。
type Registry struct {
	mu         sync.Mutex
	sealed     bool
	modules    map[string]struct{}
	components []Component
	names      map[string]struct{}
}

// NewRegistry 构建一个空的、未 Seal 的 Registry。
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]struct{}),
		names:   make(map[string]struct{}),
	}
}

// Registry 装配错误。用 errors.Is 区分。
var (
	// ErrRegistrySealed 在 Seal 后尝试变更时返回。
	ErrRegistrySealed = errors.New("runtime: registry sealed")
	// ErrDuplicateModule 由 RegisterModule 在重名模块时返回。
	ErrDuplicateModule = errors.New("runtime: 重复的模块名")
	// ErrNilModule 由 RegisterModule 在 nil 模块时返回。
	ErrNilModule = errors.New("runtime: nil 模块")
)

// RegisterModule 执行 m.Register，并记录 m 的名字用于唯一性。nil 或重名模块被拒
// 绝。若 m.Register 失败，错误用模块名包装后返回；后续模块不再运行。
//
// RegisterModule 必须在 Seal 之前调用。
//
// 执行 m.Register 时不持有 r.mu，使模块可以合法地回调 RegisterComponent（甚至为
// 子模块回调 RegisterModule）而不会自死锁。在所有模块注册完成前 Seal 被阻塞。
func (r *Registry) RegisterModule(m Module) error {
	if m == nil {
		return ErrNilModule
	}
	name := m.Name()
	if name == "" {
		return ErrEmptyComponentName
	}
	r.mu.Lock()
	if r.sealed {
		r.mu.Unlock()
		return ErrRegistrySealed
	}
	if _, exists := r.modules[name]; exists {
		r.mu.Unlock()
		return ErrDuplicateModule
	}
	r.modules[name] = struct{}{}
	r.mu.Unlock()

	// 在不持有 r.mu 的情况下执行模块注册，使其可注册组件（及更多模块）而不自死锁。
	if err := m.Register(r); err != nil {
		return &moduleError{module: name, err: err}
	}
	return nil
}

// RegisterComponent 添加一个组件。重名、nil 组件、空名字与 Seal 后调用均被拒绝。
// 注册顺序定义 Start 顺序。
func (r *Registry) RegisterComponent(c Component) error {
	if c == nil {
		return ErrNilComponent
	}
	name := c.Name()
	if name == "" {
		return ErrEmptyComponentName
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return ErrRegistrySealed
	}
	if _, exists := r.names[name]; exists {
		return ErrDuplicateComponent
	}
	r.names[name] = struct{}{}
	r.components = append(r.components, c)
	return nil
}

// Seal 冻结 registry。Seal 后不得再添加模块或组件。Seal 幂等。它返回不可变、有序
// 的组件切片（调用方通常是 App.Run，据此驱动 Start）。
func (r *Registry) Seal() ([]Component, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
	// 返回防御性副本，使后续对 slice header 的意外变更无法重排 Start。
	out := make([]Component, len(r.components))
	copy(out, r.components)
	return out, nil
}

// IsSealed 上报是否已调用 Seal。
func (r *Registry) IsSealed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sealed
}

// moduleError 用出错的模块名包装注册错误。
type moduleError struct {
	module string
	err    error
}

func (e *moduleError) Error() string { return "runtime: module " + e.module + ": " + e.err.Error() }
func (e *moduleError) Unwrap() error { return e.err }
