// Package appconfig 提供不可变、原子发布的配置存储，并支持安全的局部重载。
//
// 该 store 取代遗留的“原地修改包级全局”模式：
//
//   - Load 返回全新配置对象；发布后的快照不可变。
//   - Reload 解析候选、校验它、只合并白名单内可热更字段，并原子交换快照。需要重启
//     的字段被上报而非应用。
//   - 读者调用 Current() 得到一个可并发读取、无需任何锁的指针；store 绝不在发布后
//     原地修改它。
//
// store 对具体配置类型 T 泛型，使每个服务能用自有配置 struct，无需反射或 DI。
package appconfig

import (
	"context"
	"errors"
	"sync/atomic"
)

// Loader 从其来源（文件、远程……）构建一份全新配置对象。每次调用都必须返回新值；
// store 绝不在发布后原地修改其结果。
type Loader[T any] func(ctx context.Context) (*T, error)

// Merger 把候选配置折叠进 effective 配置。它接收先前 effective 快照的深拷贝
// （oldConfig）与刚解析的候选（candidate）。它必须：
//
//   - 只把候选中可热更（白名单）字段拷进 effective 结果。
//   - 把需重启字段保留旧值。
//   - 返回差异但未被应用的字段名列表，使调用方能上报 restart_required 诊断。
//
// 返回的 effective 指针会被原子发布；它不得别名 candidate 或 oldConfig（store 在
// 调用前深拷贝 oldConfig，并在合并后丢弃 candidate）。
type Merger[T any] func(oldConfig, candidate *T) (effective *T, restartRequired []string, err error)

// Store 持有一份原子发布的不可变配置快照。读者用 Current()；单一写者用 Load（初
// 始）与 Reload（后续）。
//
// 零值不可用；请用 New。
type Store[T any] struct {
	current atomic.Pointer[T]
	loader  Loader[T]
	merger  Merger[T]
}

// New 用给定 loader 与可选 merger 构建 Store。无 merger 时，Reload 应用整个候选
// （不做白名单过滤）；提供 Merger 可获得 restart_required 语义。
func New[T any](loader Loader[T], merger Merger[T]) *Store[T] {
	return &Store[T]{loader: loader, merger: merger}
}

// Current 返回当前不可变快照，首次成功 Load 之前为 nil。返回的指针并发读安全，
// 且绝不被 store 原地修改。
func (s *Store[T]) Current() *T {
	if s == nil {
		return nil
	}
	return s.current.Load()
}

// Load 执行初始加载并发布快照。若快照已发布则失败（后续更新用 Reload），或 loader
// 出错。出错时不发布任何快照。
func (s *Store[T]) Load(ctx context.Context) error {
	if s == nil {
		return ErrNilStore
	}
	if s.current.Load() != nil {
		return ErrAlreadyLoaded
	}
	cfg, err := s.loader(ctx)
	if err != nil {
		return err
	}
	if cfg == nil {
		return ErrNilConfig
	}
	s.current.Store(cfg)
	return nil
}

// ReloadResult 描述一次 Reload 的结果：哪些字段被应用、哪些需要重启。仅上报字段
// 名、绝不上报值，避免把敏感配置泄入日志。
type ReloadResult[T any] struct {
	Effective       *T
	Applied         []string
	RestartRequired []string
}

// Reload 解析候选、经 merger 校验，并原子交换快照。交换仅在 merger 成功后发生，
// 故失败的合并保持先前快照不变（无部分提交）。
//
// 若未配置 merger，候选原样发布（仍为原子）。配置 merger 时，只取候选的白名单字
// 段；restart_required 列出被忽略的差异字段。
//
// Reload 对并发调用安全（内部串行），且与 Current() 读者并发安全。
func (s *Store[T]) Reload(ctx context.Context) (*ReloadResult[T], error) {
	if s == nil {
		return nil, ErrNilStore
	}
	if s.loader == nil {
		return nil, ErrNoLoader
	}
	candidate, err := s.loader(ctx)
	if err != nil {
		return nil, err
	}
	if candidate == nil {
		return nil, ErrNilConfig
	}

	old := s.current.Load()
	// 无先前快照：行为同 Load。
	if old == nil {
		s.current.Store(candidate)
		return &ReloadResult[T]{Effective: candidate}, nil
	}

	if s.merger == nil {
		// 未配置白名单：原样发布候选。
		s.current.Store(candidate)
		return &ReloadResult[T]{Effective: candidate}, nil
	}

	effective, restartRequired, err := s.merger(old, candidate)
	if err != nil {
		return nil, err
	}
	if effective == nil {
		return nil, ErrNilMerge
	}
	s.current.Store(effective)
	return &ReloadResult[T]{
		Effective:       effective,
		RestartRequired: restartRequired,
	}, nil
}

// 哨兵错误。用 errors.Is 区分。
var (
	// ErrNilStore 在 nil receiver 上调用 Store 方法时返回。
	ErrNilStore = errors.New("appconfig: nil store")
	// ErrAlreadyLoaded 由 Load 在快照已发布时返回。
	ErrAlreadyLoaded = errors.New("appconfig: store 已加载；请用 Reload")
	// ErrNilConfig 在 loader 返回 nil 指针时返回。
	ErrNilConfig = errors.New("appconfig: loader 返回 nil 配置")
	// ErrNilMerge 在 merger 返回 nil effective 指针时返回。
	ErrNilMerge = errors.New("appconfig: merger 返回 nil effective 配置")
	// ErrNoLoader 在无 loader 的 store 上调用 Reload 时返回。
	ErrNoLoader = errors.New("appconfig: store 无 loader")
)
