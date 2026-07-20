// Package runtime 提供 GoOne 服务统一的应用生命周期。
//
// 一个服务由若干 Component 装配而成，由单次 App.Run 驱动。每个运行时资源
// （监听器、bus consumer、事务分片、调度器、admin server）都必须归属到唯一
// 一个 Component，使启动、排空与关停始终可追踪。
//
// # Component 契约
//
//   - Name 返回在 App 内唯一的标识。重复名字会在 Register 阶段（早于任何
//     Start 调用）被拒绝。
//   - Start 仅在组件可用时返回 nil。失败时由组件自身清理其部分初始化状态；
//     App 不会对 Start 失败的组件调用 Stop。
//   - Stop 必须幂等、遵守其 context、并等待组件自身 goroutine 退出。
//   - 组件内严禁调用 os.Exit 或 logger.Fatalf；致命决策属于 Run 的调用方。
//
// # 可选生命周期钩子
//
// 组件可实现 Quiescer 来停止接收新工作（关闭监听器、拒绝新会话），实现
// Drainer 来等待在途工作完成。Quiesce 永远先于 Drain 执行，且两者都先于
// Stop，按注册逆序执行。
//
//	Starting -> Ready -> Draining -> Stopping -> Stopped
//
// 完整状态机、观察者与 admin 端点位于本包，由 State 类型描述。
//
// # goroutine 归属
//
// 每个 goroutine 必须归属到某个组件，且在该组件的 Stop（或 Drain）context
// 取消时退出。App.Run 只有在所有组件 goroutine 都 join 后才返回。
package runtime
