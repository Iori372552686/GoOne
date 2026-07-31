package async

import (
	"context"
	"errors"
	"fmt"
)

// WorkerComponent 把一组 Async worker 包装成 runtime.Component，使异步任务队列的生命周期
// 由运行期统一管理。
//
// 它实现 Start、Quiesce、Drain、Stop：
//   - Start：启动所有 worker goroutine；
//   - Quiesce：停止接收新任务（PushE 返回 ErrWorkerQuiescing）；
//   - Drain：等待所有 worker 队列归零或 ctx 取消；
//   - Stop：停止 worker 并 join goroutine。
//
// 该 Component 只持有自己的 []*Async，不引用服务全局变量。
//
// 注意：为避免 import 循环，本 Component 不直接实现 runtime.Component 接口签名依赖，
// 但方法签名与 runtime.Component / Quiescer / Drainer 完全一致，可被 Runtime 直接装配。
type WorkerComponent struct {
	name    string
	workers []*Async
}

// NewWorkerComponent 构造一个 worker Component。name 为组件名，workers 为其持有的池
// （通常由 NewAsyncPool / NewAsyncPoolWithCapacity 构造）。
func NewWorkerComponent(name string, workers []*Async) *WorkerComponent {
	return &WorkerComponent{name: name, workers: workers}
}

// Name 返回组件名。
func (w *WorkerComponent) Name() string { return w.name }

// Start 启动所有 worker goroutine。
func (w *WorkerComponent) Start(_ context.Context) error {
	for _, a := range w.workers {
		a.Start()
	}
	return nil
}

// Quiesce 标记所有 worker 进入排空期（停止接收新任务）。
func (w *WorkerComponent) Quiesce(_ context.Context) error {
	for _, a := range w.workers {
		a.Quiesce()
	}
	return nil
}

// Drain 等待所有 worker 队列归零或 ctx 取消。任一 worker 在 ctx 取消时仍有残余任务，
// 返回聚合 error（持久化失败必须使 Drain 返回 error）。
func (w *WorkerComponent) Drain(ctx context.Context) error {
	var errs []error
	for i, a := range w.workers {
		if err := a.Drain(ctx); err != nil {
			errs = append(errs, fmt.Errorf("async worker %s[%d] drain: %w", w.name, i, err))
		}
	}
	return errors.Join(errs...)
}

// Stop 停止所有 worker 并 join。幂等。
func (w *WorkerComponent) Stop(_ context.Context) error {
	for _, a := range w.workers {
		a.Stop()
	}
	return nil
}

// Push 向指定 worker（按 index）投递任务，返回该 worker 的 PushE 错误。
func (w *WorkerComponent) Push(index int, task func()) error {
	if index < 0 || index >= len(w.workers) {
		return fmt.Errorf("async worker %s: index %d out of range [0,%d)", w.name, index, len(w.workers))
	}
	return w.workers[index].PushE(task)
}
