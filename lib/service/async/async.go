package async

import (
	"sync"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/util/safego"
)

const (
	STATUS_RUN  = 1
	STATUS_STOP = 2
)

type Async struct {
	sync.WaitGroup
	status int32         // actor运行状态
	tasks  *Queue        // 任务队列
	pushCh chan struct{} // 消耗通知
	exitCh chan struct{} // 退出
}

func NewAsync() *Async {
	return &Async{
		status: STATUS_STOP,
		tasks:  NewQueue(),
		pushCh: make(chan struct{}, 1),
		exitCh: make(chan struct{}, 0),
	}
}

func NewAsyncPool(size int) (rets []*Async) {
	for i := 0; i < size; i++ {
		rets = append(rets, NewAsync())
	}
	return
}

// 添加任务
func (d *Async) Push(task func()) {
	if atomic.CompareAndSwapInt32(&d.status, STATUS_RUN, STATUS_RUN) {
		d.tasks.Push(task)
		// 避免阻塞
		select {
		case d.pushCh <- struct{}{}:
		default:
		}
	}
}

// 开始actor任务协程
func (d *Async) Start() {
	if atomic.CompareAndSwapInt32(&d.status, STATUS_RUN, STATUS_RUN) {
		return
	}
	atomic.StoreInt32(&d.status, STATUS_RUN)
	d.Add(1)
	go d.run()
}

// 停止actor任务协程
func (d *Async) Stop() {
	if atomic.CompareAndSwapInt32(&d.status, STATUS_STOP, STATUS_STOP) {
		return
	}
	atomic.StoreInt32(&d.status, STATUS_STOP)
	// 等待停止
	d.exitCh <- struct{}{}
	d.Wait()
}

func (d *Async) run() {
	defer func() {
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
