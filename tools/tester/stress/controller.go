// Package stress 全流程压力测试引擎。
//
// Controller 负责全局控制：梯次上线、运行时增减玩家、暂停/恢复、
// 到时或手动停止后优雅收尾；每个玩家一个 worker 协程。
package stress

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
)

// Controller 压测全局控制器。
type Controller struct {
	cfg       *testcfg.Config
	collector *stats.Collector
	modules   []string // 已过滤：enabled 且实现 StressRunner

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	workers  map[int]*worker
	nextSlot int

	paused atomic.Bool

	stopOnce   sync.Once
	stopReason atomic.Value // string
	doneCh     chan struct{}
}

// NewController 创建控制器；会过滤掉未实现 StressRunner 的模块并告警。
func NewController(cfg *testcfg.Config, collector *stats.Collector) (*Controller, error) {
	var capable []string
	for _, name := range cfg.EnabledModules() {
		comp, err := component.Create(name)
		if err != nil {
			return nil, fmt.Errorf("module %q not registered: %w", name, err)
		}
		if _, ok := comp.(component.StressRunner); ok {
			capable = append(capable, name)
		} else {
			log.Printf("[Stress] module %q has no stress operations (StressRunner not implemented), skipped", name)
		}
	}
	if len(capable) == 0 {
		return nil, fmt.Errorf("no enabled module implements StressRunner; check [modules] config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Controller{
		cfg:       cfg,
		collector: collector,
		modules:   capable,
		ctx:       ctx,
		cancel:    cancel,
		workers:   make(map[int]*worker),
		doneCh:    make(chan struct{}),
	}, nil
}

// Modules 返回实际参与压测的模块列表。
func (c *Controller) Modules() []string { return c.modules }

// Start 按配置拉起初始玩家；duration > 0 时到时自动停止。非阻塞。
func (c *Controller) Start() {
	c.AddPlayers(c.cfg.Player.Players)

	if d := c.cfg.Stress.DurationParsed(); d > 0 {
		go func() {
			select {
			case <-c.ctx.Done():
			case <-time.After(d):
				c.Stop("到达配置时长")
			}
		}()
	}
}

// AddPlayers 增加 n 个玩家，按 ramp_up_per_sec 梯次上线。
func (c *Controller) AddPlayers(n int) {
	if n <= 0 {
		return
	}
	rate := c.cfg.Stress.RampUpPerSec
	if rate <= 0 {
		rate = 20
	}
	interval := time.Second / time.Duration(rate)

	c.mu.Lock()
	slots := make([]int, 0, n)
	for i := 0; i < n; i++ {
		slot := c.nextSlot
		c.nextSlot++
		w := newWorker(slot, c)
		c.workers[slot] = w
		slots = append(slots, slot)
	}
	c.mu.Unlock()

	go func() {
		for _, slot := range slots {
			if c.ctx.Err() != nil {
				return
			}
			c.mu.Lock()
			w := c.workers[slot]
			c.mu.Unlock()
			if w == nil {
				continue // 已被 RemovePlayers 移除
			}
			go func(w *worker) {
				w.run(c.ctx)
				c.mu.Lock()
				delete(c.workers, w.slot)
				c.mu.Unlock()
			}(w)

			select {
			case <-c.ctx.Done():
				return
			case <-time.After(interval):
			}
		}
	}()
	log.Printf("[Stress] adding %d players (ramp-up %d/s)", n, rate)
}

// RemovePlayers 移除 n 个玩家（槽位号最大的优先）。
func (c *Controller) RemovePlayers(n int) {
	if n <= 0 {
		return
	}
	c.mu.Lock()
	slots := make([]int, 0, len(c.workers))
	for slot := range c.workers {
		slots = append(slots, slot)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(slots)))
	if n > len(slots) {
		n = len(slots)
	}
	victims := make([]*worker, 0, n)
	for _, slot := range slots[:n] {
		victims = append(victims, c.workers[slot])
		delete(c.workers, slot)
	}
	c.mu.Unlock()

	for _, w := range victims {
		if w.cancel != nil {
			w.cancel()
		}
	}
	log.Printf("[Stress] removing %d players", len(victims))
}

// Pause 暂停业务循环（连接与登录态保持）。
func (c *Controller) Pause() {
	c.paused.Store(true)
	log.Printf("[Stress] paused")
}

// Resume 恢复业务循环。
func (c *Controller) Resume() {
	c.paused.Store(false)
	log.Printf("[Stress] resumed")
}

// Paused 报告是否处于暂停状态。
func (c *Controller) Paused() bool { return c.paused.Load() }

// WorkerCount 当前存活的 worker 数（含未完成登录的）。
func (c *Controller) WorkerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.workers)
}

// MaxSlot 分配过的最大槽位 +1（用于报告 UID 段）。
func (c *Controller) MaxSlot() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nextSlot
}

// Stop 优雅停止：取消所有 worker 并等待退出（最多 10s）。可重复调用。
func (c *Controller) Stop(reason string) {
	c.stopOnce.Do(func() {
		c.stopReason.Store(reason)
		log.Printf("[Stress] stopping: %s", reason)
		c.cancel()

		go func() {
			deadline := time.After(10 * time.Second)
			tick := time.NewTicker(100 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-deadline:
					close(c.doneCh)
					return
				case <-tick.C:
					if c.WorkerCount() == 0 {
						close(c.doneCh)
						return
					}
				}
			}
		}()
	})
}

// StopReason 返回停止原因；未停止时为空串。
func (c *Controller) StopReason() string {
	if v, ok := c.stopReason.Load().(string); ok {
		return v
	}
	return ""
}

// Done 停止收尾完成的信号。
func (c *Controller) Done() <-chan struct{} { return c.doneCh }

// Ctx 控制器根上下文（供采集器等派生）。
func (c *Controller) Ctx() context.Context { return c.ctx }
