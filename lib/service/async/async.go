package async

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/util/safego"
)

const (
	STATUS_RUN     = 1
	STATUS_STOP    = 2
	STATUS_QUIESCE = 3 // Quiesce 后停止接收新任务，既有任务继续完成。

	// defaultCapacity 是未显式设置容量时的上限。避免历史无界增长。
	defaultCapacity int64 = 1 << 16
)

// Push 失败原因的哨兵错误。
var (
	// ErrWorkerStopped 表示 worker 已 Stop，Push 被拒绝。
	ErrWorkerStopped = errors.New("async: worker stopped")
	// ErrWorkerQuiescing 表示 worker 处于 Quiesce（排空期），Push 被拒绝。
	ErrWorkerQuiescing = errors.New("async: worker quiescing")
	// ErrQueueFull 表示队列已满（达到容量上限）。
	ErrQueueFull = errors.New("async: queue full")
)

type Async struct {
	sync.WaitGroup
	status   int32         // actor 运行状态：RUN / STOP / QUIESCE
	tasks    *Queue        // 任务队列
	pushCh   chan struct{} // 消耗通知
	exitCh   chan struct{} // 退出
	capacity int64         // 队列容量上限，<=0 用 defaultCapacity
}

// NewAsync 构造一个默认容量（defaultCapacity）的 worker。
func NewAsync() *Async {
	return NewAsyncWithCapacity(defaultCapacity)
}

// NewAsyncWithCapacity 构造一个显式容量的 worker。capacity<=0 时用默认值。
func NewAsyncWithCapacity(capacity int64) *Async {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Async{
		status:   STATUS_STOP,
		tasks:    NewQueue(),
		pushCh:   make(chan struct{}, 1),
		exitCh:   make(chan struct{}, 0),
		capacity: capacity,
	}
}

// NewAsyncPool 构造一个默认容量的 worker 切片。
func NewAsyncPool(size int) (rets []*Async) {
	for i := 0; i < size; i++ {
		rets = append(rets, NewAsync())
	}
	return
}

// NewAsyncPoolWithCapacity 构造一个显式容量的 worker 切片。
func NewAsyncPoolWithCapacity(size int, capacity int64) (rets []*Async) {
	for i := 0; i < size; i++ {
		rets = append(rets, NewAsyncWithCapacity(capacity))
	}
	return
}

// Push 添加任务。Deprecated：保留兼容签名，内部调用 PushE 并丢弃 error。
// 新代码应使用 PushE 以获得 stopped/quiescing/queue full 的明确错误。
func (d *Async) Push(task func()) {
	_ = d.PushE(task)
}

// PushE 添加任务并返回错误。至少区分：
//   - ErrWorkerStopped：worker 已 Stop；
//   - ErrWorkerQuiescing：worker 处于 Quiesce；
//   - ErrQueueFull：队列达到容量上限（有界，不再无界增长）。
func (d *Async) PushE(task func()) error {
	switch atomic.LoadInt32(&d.status) {
	case STATUS_RUN:
		// 继续；容量检查在下方。
	default:
		if atomic.LoadInt32(&d.status) == STATUS_QUIESCE {
			return ErrWorkerQuiescing
		}
		return ErrWorkerStopped
	}
	// 容量检查：超过上限拒绝，避免无界增长。
	if d.tasks.GetCount() >= d.capacity {
		return ErrQueueFull
	}
	d.tasks.Push(task)
	// 非阻塞通知 worker。
	select {
	case d.pushCh <- struct{}{}:
	default:
	}
	return nil
}

// 开始 actor 任务协程
func (d *Async) Start() {
	if !atomic.CompareAndSwapInt32(&d.status, STATUS_STOP, STATUS_RUN) {
		return
	}
	d.Add(1)
	go d.run()
}

// Quiesce 标记 worker 进入排空期：停止接收新任务（PushE 返回 ErrWorkerQuiescing），
// 但既有任务继续完成。幂等。
func (d *Async) Quiesce() {
	atomic.CompareAndSwapInt32(&d.status, STATUS_RUN, STATUS_QUIESCE)
}

// Drain 阻塞等待队列排空或 ctx 取消。仅在 Quiesce 后调用语义明确。
// 返回 nil 表示队列已归零；返回 ctx.Err() 表示等待被取消时仍有残余任务。
func (d *Async) Drain(ctx context.Context) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if d.tasks.GetCount() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// 停止 actor 任务协程
func (d *Async) Stop() {
	if !atomic.CompareAndSwapInt32(&d.status, STATUS_RUN, STATUS_STOP) {
		// 可能处于 QUIESCE；统一置为 STOP。
		if !atomic.CompareAndSwapInt32(&d.status, STATUS_QUIESCE, STATUS_STOP) {
			return
		}
	}
	// 等待停止
	d.exitCh <- struct{}{}
	d.Wait()
}

func (d *Async) run() {
	defer func() {
		// 退出前排空残余任务（尽力完成已入队任务）。
		for data := d.tasks.Pop(); data != nil; data = d.tasks.Pop() {
			safego.SafeFunc(data.(func()))
		}
		d.Done()
	}()
	for {
		select {
		case <-d.pushCh:
			for data := d.tasks.Pop(); data != nil; data = d.tasks.Pop() {
				safego.SafeFunc(data.(func()))
			}
		case <-d.exitCh:
			return
		}
	}
}

// Capacity 返回队列容量上限（测试与指标观测用）。
func (d *Async) Capacity() int64 { return d.capacity }

// QueueDepth 返回当前队列深度（测试与指标观测用）。
func (d *Async) QueueDepth() int64 { return d.tasks.GetCount() }
