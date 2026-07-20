package runtime

import "context"

// Component 是 App 中运行时资源拥有的基本单元。
//
// 完整契约见包文档。方法刻意保持精简：Start/Stop 为必选；Quiesce 与 Drain
// 通过 Quiescer、Drainer 接口可选实现。
type Component interface {
	// Name 在 App 内唯一，用于日志、指标与 admin /components 端点。
	Name() string
	// Start 使组件可用。仅当其拥有的资源（监听器、consumer、调度器……）真正
	// 就绪时才返回 nil。失败时组件必须自行清理部分初始化状态；App 不会对
	// Start 失败的组件调用 Stop。
	Start(ctx context.Context) error
	// Stop 释放组件资源。必须幂等、遵守 ctx、并等待组件 goroutine 退出。
	Stop(ctx context.Context) error
}

// Quiescer 由能“停止接收新工作但不拆除”的组件实现。Quiesce 在 Drain 与
// Stop 之前（按注册逆序）执行。典型用途：关闭监听器、拒绝新登录、翻转
// admission gate。
//
// Quiesce 与 Drain 的区别：Quiesce 拒绝新工作，Drain 等待在途工作。网关通常
// 两者都实现；后台调度器通常只实现 Stop。
type Quiescer interface {
	Quiesce(ctx context.Context) error
}

// Drainer 由“存在在途工作需要等待”的组件实现。Drain 在 Quiesce 之后、Stop
// 之前（按注册逆序）执行，受 App 排空超时约束。超时的 Drain 会被取消，Stop
// 照常进行。
type Drainer interface {
	Drain(ctx context.Context) error
}

// ComponentFunc 是把简单闭包适配为 Component 的工具。用于测试和小型胶水组件
// （无需专门 struct）。Name 固定；Start 与 Stop 为传入的闭包。Start 不得为
// nil；Stop 可为 nil（按 no-op 处理）。
type ComponentFunc struct {
	ComponentName string
	OnStart       func(ctx context.Context) error
	OnStop        func(ctx context.Context) error
}

// Name 实现 Component。
func (c ComponentFunc) Name() string { return c.ComponentName }

// Start 实现 Component。
func (c ComponentFunc) Start(ctx context.Context) error {
	if c.OnStart == nil {
		return nil
	}
	return c.OnStart(ctx)
}

// Stop 实现 Component。OnStop 为 nil 时视为成功，使只读或 fire-and-forget 组件
// 可以省略它。
func (c ComponentFunc) Stop(ctx context.Context) error {
	if c.OnStop == nil {
		return nil
	}
	return c.OnStop(ctx)
}

// quiescerOf 返回 c 的 Quiescer 视图，若 c 未实现则为 nil。
func quiescerOf(c Component) Quiescer {
	if q, ok := c.(Quiescer); ok {
		return q
	}
	return nil
}

// drainerOf 返回 c 的 Drainer 视图，若 c 未实现则为 nil。
func drainerOf(c Component) Drainer {
	if d, ok := c.(Drainer); ok {
		return d
	}
	return nil
}
