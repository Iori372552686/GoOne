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
	"sync"
	"sync/atomic"
)

// Loader 从其来源（文件、远程……）构建一份全新配置对象。每次调用都必须返回新值；
// store 绝不在发布后原地修改其结果。
type Loader[T any] func(ctx context.Context) (*T, error)

// MergeResult 是 Merger 的显式结果（P1-06）。Effective 是合并后的不可变快照（不得
// 别名 old/candidate，必须深拷贝其 map/slice）；Applied 是本次真正被热更应用的字段
// 名；RestartRequired 是差异但未应用、需重启才生效的字段名。
type MergeResult[T any] struct {
	Effective       *T
	Applied         []string
	RestartRequired []string
}

// Merger 把候选配置折叠进 effective 配置。它接收先前的 effective 快照（oldConfig）
// 与刚解析的候选（candidate）。
//
// P1-06 契约修正：
//   - Merger 必须构造一个**不别名** old/candidate 的新 effective 对象，并深拷贝其
//     map/slice。Store **不**自动深拷贝 oldConfig（历史文档谎称 store 会深拷贝）。
//   - 只把候选中可热更（白名单）字段拷进 effective 结果；需重启字段保留旧值。
//   - 返回 Applied（被应用字段）与 RestartRequired（被忽略的差异字段），使调用方能
//     上报诊断。仅上报字段名、绝不上报值。
type Merger[T any] func(oldConfig, candidate *T) (MergeResult[T], error)

// Store 持有一份原子发布的不可变配置快照。读者用 Current()；单一写者用 Load（初
// 始）与 Reload（后续）。
//
// writeMu 串行化 Load/Reload，使并发 Load 只发布一次、并发 Reload 顺序确定；
// Current() 仍走 atomic 无锁读。
//
// 零值不可用；请用 New。
//
// Deprecated: appconfig.Store 是通用配置热更抽象，但 GoOne 生产配置采用启动不可变
// 模型（仅 gamedata 热更）。当前无生产代码使用 Store；通用 Reload 能力将在下一主
// 版本删除（V3-P1-04）。需要配置读取的服务应通过构造参数接收已 Normalize/Validate
// 的配置快照。
type Store[T any] struct {
	current atomic.Pointer[T]
	writeMu sync.Mutex
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
//
// P1-06：writeMu 串行化，使并发 Load 只有一个成功发布，另一个返回 ErrAlreadyLoaded
//（历史 current.Load() 检查存在 TOCTOU，并发 Load 可能双发）。
func (s *Store[T]) Load(ctx context.Context) error {
	if s == nil {
		return ErrNilStore
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
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
// 段；Applied/RestartRequired 分别列出被应用与需重启的字段。
//
// P1-06：writeMu 串行化并发 Reload（后一次基于前一次 effective）；Current() 仍无锁读。
func (s *Store[T]) Reload(ctx context.Context) (*ReloadResult[T], error) {
	if s == nil {
		return nil, ErrNilStore
	}
	if s.loader == nil {
		return nil, ErrNoLoader
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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

	result, err := s.merger(old, candidate)
	if err != nil {
		return nil, err
	}
	if result.Effective == nil {
		return nil, ErrNilMerge
	}
	s.current.Store(result.Effective)
	return &ReloadResult[T]{
		Effective:       result.Effective,
		Applied:         result.Applied,
		RestartRequired: result.RestartRequired,
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
