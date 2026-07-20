// Package timer 提供层级时间轮（Hierarchical Timing Wheel），适用于海量延时任务
// 调度，如帧同步的技能 CD、buff 到期、定时邮件、连接超时、延迟队列等。
//
// 设计要点：
//   - 多层级级联：低层指针转一圈（归零）时，从高层对应槽位取任务"降级"重新分配到
//     低层，从而以 O(1) 添加支持任意大延迟，避免单层轮"转很多圈"的浪费。
//   - 全程整数运算：用 time.Duration 而非 float64.Seconds()，亚秒级延迟精度无损
//     （旧实现的浮点转换会让 150ms 之类的延迟全错）。
//   - 单调度线程：所有槽位操作在后台 goroutine 内串行执行，无锁；外部 AddTimer /
//     RemoveTimer 经带缓冲 channel 投递请求，绝不阻塞调用方。
//   - Stop 幂等无死锁：用 context + sync.Once，不会像旧实现那样向已退出的 goroutine
//     阻塞发送。
//   - job panic 隔离：每个任务回调带 recover，单个 job 崩溃不影响后续 tick。
//
// 注意：本包位于 lib/util 工具层，不实现 runtime.Component（避免反向依赖 service 层）。
// 需要生命周期托管时，由调用方用 runtime.ComponentFunc 包一层，或在新文件中组合。
package timer

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Job 延时任务回调。data 为 AddTimer 时传入的参数。
type Job func(data interface{})

// task 延时任务（内部类型，不导出）。
type task struct {
	expiration time.Duration // 相对启动时刻的绝对到期 tick 数（按 tickInterval 计）
	round      int           // 在当前层需要再转的整圈数（仅单层模式用，层级模式按层路由）
	key        interface{}   // 唯一标识，用于 RemoveTimer；nil 表示匿名任务
	data       interface{}   // 回调参数
}

// layer 时间轮的一层。每层有 slotNum 个槽，每格代表 interval 的时间。
type layer struct {
	slots   []*list.List
	slotNum int
	// perSlot 本层一格代表的时间（= tickInterval × 前层总容量）。
	// L1.perSlot = tickInterval；L2.perSlot = tickInterval × L1.slotNum；以此类推。
	perSlot time.Duration
	// capacity 本层总容量（= perSlot × slotNum），即本层能直接表达的最大延迟。
	capacity time.Duration
	curPos   int
}

// TimeWheel 层级时间轮。
type TimeWheel struct {
	tickInterval time.Duration
	layers       []*layer
	job          Job
	asyncJob     bool // true: job 在独立 goroutine 执行（默认 false 同步）

	// 与外部交互的 channel（单调度线程消费，避免锁）。
	addCh    chan *task
	removeCh chan interface{}
	tickCh   chan time.Duration // 投递一次 tick 推进量（= tickInterval），便于测试注入

	cancel   context.CancelFunc
	stopOnce sync.Once
	stopped  atomic.Bool

	// key -> 任务所在 (layerIdx, slotIdx)，仅调度线程访问，无需加锁。
	loc map[interface{}][2]int

	// cancelled 记录已被 Remove 但可能尚未被 schedule 或仍在槽中的 key。
	// 解决 add/remove 经两个独立 channel 投递的乱序问题：Remove 先于 Add 被消费时，
	// loc 里还没有该 key，但 cancelled 会标记它，后续 schedule/cascade/exec 时跳过。
	// 仅调度线程访问。为避免无限增长，exec/removeByKey 命中后删除对应 entry。
	cancelled map[interface{}]struct{}

	// elapsed 调度线程累计已推进的时间（按 tickInterval 倍数累加）。
	elapsed time.Duration

	// 装配期参数（由 Option 写入，buildLayers 读取）。
	slotNumCfg int
	layersCfg  int
}

// Option 配置 TimeWheel。
type Option func(*TimeWheel)

// WithTickInterval 设置最低层 tick 周期（即时间轮精度）。默认 100ms。
// 值越小精度越高但 CPU 开销越大；帧同步场景常见 16ms (~60fps) 或 33ms (~30fps)。
func WithTickInterval(d time.Duration) Option {
	return func(tw *TimeWheel) {
		if d > 0 {
			tw.tickInterval = d
		}
	}
}

// WithSlotNum 设置每层槽位数。默认 60。所有层共用同一槽位数。
// 槽位越多，单层能表达的延迟越大，层数越少；权衡内存与层数。
func WithSlotNum(n int) Option {
	return func(tw *TimeWheel) {
		if n > 0 {
			tw.slotNumCfg = n
		}
	}
}

// WithLayers 设置层级数。默认 3。
// 每加一层，最大延迟扩大 slotNum 倍。默认配置（100ms×60×3层）可表达 6min×60=6h。
func WithLayers(n int) Option {
	return func(tw *TimeWheel) {
		if n > 0 {
			tw.layersCfg = n
		}
	}
}

// WithAsyncJob 设置 job 是否在独立 goroutine 中执行。默认 false（同步）。
// 同步执行保证任务按到期顺序串行处理，适合帧同步等需要确定性时序的场景；
// 异步执行适合 job 本身较慢且互不相关的场景，但会失去顺序保证并产生无上限 goroutine。
func WithAsyncJob(b bool) Option {
	return func(tw *TimeWheel) { tw.asyncJob = b }
}

// NewTimeWheel 创建层级时间轮。必须在 Start 前配置完毕。
func NewTimeWheel(job Job, opts ...Option) *TimeWheel {
	if job == nil {
		return nil
	}
	tw := &TimeWheel{
		tickInterval: 100 * time.Millisecond,
		slotNumCfg:   60,
		layersCfg:    3,
		job:          job,
		loc:          make(map[interface{}][2]int),
		cancelled:    make(map[interface{}]struct{}),
		// channel 缓冲：避免高频 AddTimer 时调度线程暂时卡在 tick 处理而阻塞调用方。
		// 缓冲大小按经验取 1024，业务可按需调整（后续可加 Option）。
		addCh:    make(chan *task, 1024),
		removeCh: make(chan interface{}, 128),
		tickCh:   make(chan time.Duration, 16),
	}
	for _, o := range opts {
		o(tw)
	}
	tw.buildLayers()
	return tw
}

// slotNumCfg / layersCfg 由 Option 写入，buildLayers 读取；放在主结构体便于 Option 访问。

// buildLayers 按 tickInterval / slotNumCfg / layersCfg 构造各层。
func (tw *TimeWheel) buildLayers() {
	perSlot := tw.tickInterval
	for i := 0; i < tw.layersCfg; i++ {
		l := &layer{
			slotNum:  tw.slotNumCfg,
			slots:    make([]*list.List, tw.slotNumCfg),
			perSlot:  perSlot,
			capacity: perSlot * time.Duration(tw.slotNumCfg),
		}
		for j := 0; j < tw.slotNumCfg; j++ {
			l.slots[j] = list.New()
		}
		tw.layers = append(tw.layers, l)
		perSlot = l.capacity // 下一层 perSlot = 本层总容量
	}
}

// Start 启动后台调度 goroutine。幂等（多次调用仅首次生效）。返回的 ctx 可用于外部
// 等待（ctx.Done 在 Stop 后关闭）。
func (tw *TimeWheel) Start(ctx context.Context) context.Context {
	if tw.stopped.Load() {
		return ctx
	}
	ctx, cancel := context.WithCancel(ctx)
	tw.cancel = cancel
	go tw.run(ctx)
	return ctx
}

// Stop 停止时间轮。幂等、非阻塞（绝不向已退出 goroutine 阻塞发送）。
// 停止后未触发的任务不会被执行；在途的 job（asyncJob 模式）不等待。
func (tw *TimeWheel) Stop() {
	tw.stopOnce.Do(func() {
		tw.stopped.Store(true)
		if tw.cancel != nil {
			tw.cancel()
		}
	})
}

// AddTimer 添加一个延时任务。delay 为相对现在的延迟；key 为唯一标识（可用于
// RemoveTimer），传 nil 表示匿名任务（不可删除）。delay<=0 立即投递到下一 tick。
// 非阻塞：channel 缓冲满时返回 false（调用方可据此背压）。
//
// 重复 key：新任务会覆盖旧任务的定位索引，旧任务虽仍在槽中但 RemoveTimer 按 key
// 会定位到新任务；若需严格唯一，调用方自行保证。
func (tw *TimeWheel) AddTimer(delay time.Duration, key interface{}, data interface{}) bool {
	if tw.stopped.Load() {
		return false
	}
	if delay < 0 {
		delay = 0
	}
	t := &task{
		expiration: tw.elapsed + delay,
		key:        key,
		data:       data,
	}
	select {
	case tw.addCh <- t:
		return true
	default:
		// 缓冲满：拒绝，避免调用方被阻塞或调度线程被压垮。
		return false
	}
}

// RemoveTimer 删除任务。key 为 AddTimer 时传入的标识。匿名任务（key=nil）不可删。
// 非阻塞；key 不存在静默忽略。
func (tw *TimeWheel) RemoveTimer(key interface{}) {
	if key == nil || tw.stopped.Load() {
		return
	}
	select {
	case tw.removeCh <- key:
	default:
		// removeCh 满：极少见（通常 remove 远少于 add）；丢弃此次 remove 请求，
		// 任务到期仍会触发。这里选择不阻塞调用方。
	}
}

// run 调度线程主体。单 goroutine 消费 channel + 推进 tick，所有槽位操作无锁。
func (tw *TimeWheel) run(ctx context.Context) {
	ticker := time.NewTicker(tw.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tw.advance(tw.tickInterval)
		case d := <-tw.tickCh:
			// 测试注入的推进量；正常路径不走这里。
			tw.advance(d)
		case t := <-tw.addCh:
			tw.schedule(t)
		case key := <-tw.removeCh:
			tw.removeByKey(key)
		}
	}
}

// advance 推进时间并处理到期任务。delta 必须为 tickInterval 的整数倍（正常路径恒成立）。
func (tw *TimeWheel) advance(delta time.Duration) {
	tw.elapsed += delta
	// L1 推进一格，并按需级联处理高层。
	tw.tickLayer(0)
}

// tickLayer 推进第 idx 层一格。若该层指针归零，递归推进上一层（cascade）。
func (tw *TimeWheel) tickLayer(idx int) {
	l := tw.layers[idx]
	// 处理当前槽：本格内所有任务的 round==0 即到期（层级模式下 round 通常为 0，
	// 真正的长延迟任务在更高层；这里统一用 expiration 判定更稳妥）。
	tw.runSlot(idx, l.curPos)

	// 推进指针。
	l.curPos++
	if l.curPos >= l.slotNum {
		// 归零：从高层对应位置"降级"任务到本层及更低层。
		l.curPos = 0
		if idx+1 < len(tw.layers) {
			tw.tickLayer(idx + 1)
			// 高层 tickLayer 后，高层当前槽的任务需要重新分配到低层。
			tw.cascadeDown(idx + 1)
		}
	}
}

// runSlot 执行第 idx 层 slotIdx 槽中所有到期任务，未到期的留下。
// 层级模式下，低层（idx=0）槽内任务到期判定用 expiration <= elapsed。
func (tw *TimeWheel) runSlot(idx int, slotIdx int) {
	l := tw.layers[idx]
	slot := l.slots[slotIdx]
	for e := slot.Front(); e != nil; {
		next := e.Next()
		t := e.Value.(*task)
		if t.expiration <= tw.elapsed {
			slot.Remove(e)
			// 到期执行前最后检查 cancelled：Remove 可能在任务到期前的任意时刻到达。
			if t.key != nil {
				delete(tw.loc, t.key)
				if _, ok := tw.cancelled[t.key]; ok {
					delete(tw.cancelled, t.key)
					e = next
					continue
				}
			}
			tw.exec(t)
		}
		e = next
	}
}

// cascadeDown 把第 idx 层当前槽的任务重新分配到 idx-1 及更低层。
// 在第 idx 层指针归零（刚被 tickLayer 推进）后调用：这些任务原本挂在"高层某个刻度"，
// 现在该刻度到来，应把它们展开到低层的具体格子里。
func (tw *TimeWheel) cascadeDown(idx int) {
	if idx <= 0 {
		return
	}
	l := tw.layers[idx]
	slot := l.slots[l.curPos]
	// 取出全部，逐个重新 schedule 到更低层。
	var pending []*task
	for e := slot.Front(); e != nil; e = e.Next() {
		pending = append(pending, e.Value.(*task))
	}
	slot.Init() // 清空当前槽
	for _, t := range pending {
		if t.key != nil {
			delete(tw.loc, t.key)
			// 级联时再次检查 cancelled（Remove 可能在任务挂到高层后、级联前到达）。
			if _, ok := tw.cancelled[t.key]; ok {
				delete(tw.cancelled, t.key)
				continue
			}
		}
		// 剩余延迟 = expiration - elapsed，重新路由到 0..idx-1 层。
		tw.scheduleIn(t, idx-1)
	}
}

// schedule 把任务路由到合适的层与槽。遍历从低到高，找到第一个 capacity >= 剩余延迟
// 的层；若所有层都不够（理论上不应发生，因 expiration 不会超过最高层 capacity），
// 放到最高层当前槽（下一轮再处理）。
func (tw *TimeWheel) schedule(t *task) {
	remaining := t.expiration - tw.elapsed
	if remaining <= 0 {
		// 已到期：直接执行（仍交给 exec，保持 panic 隔离）。
		tw.exec(t)
		return
	}
	tw.scheduleIn(t, len(tw.layers)-1)
}

// scheduleIn 在 [0..maxIdx] 范围内为任务选层挂载。
func (tw *TimeWheel) scheduleIn(t *task, maxIdx int) {
	// 若在 add 投递后、schedule 前被 Remove，直接丢弃。
	if t.key != nil {
		if _, ok := tw.cancelled[t.key]; ok {
			delete(tw.cancelled, t.key)
			return
		}
	}
	remaining := t.expiration - tw.elapsed
	if remaining < 0 {
		remaining = 0
	}
	// 选层：第一个 capacity > remaining 的层（用 > 而非 >=，保证恰好等于 capacity 时
	// 落到下一层，避免边界格刚好本轮触发）。
	chosen := 0
	for i := 0; i <= maxIdx; i++ {
		if tw.layers[i].capacity > remaining {
			chosen = i
			break
		}
		chosen = i
	}
	l := tw.layers[chosen]
	// 该层需要推进的格数 = remaining / perSlot（向下取整）。
	ticks := int64(remaining / l.perSlot)
	pos := (l.curPos + int(ticks)) % l.slotNum
	l.slots[pos].PushBack(t)
	if t.key != nil {
		tw.loc[t.key] = [2]int{chosen, pos}
	}
}

// removeByKey 调度线程内执行：按 key 摘除已挂载任务；若尚未挂载（add 还没被消费）
// 或在级联途中，则记入 cancelled，由 schedule/runSlot/cascadeDown 兜底跳过。
func (tw *TimeWheel) removeByKey(key interface{}) {
	// 无论如何先标记：覆盖"add 尚未消费"与"级联中转"两种乱序场景。
	tw.cancelled[key] = struct{}{}
	loc, ok := tw.loc[key]
	if !ok {
		return
	}
	delete(tw.loc, key)
	l := tw.layers[loc[0]]
	slot := l.slots[loc[1]]
	for e := slot.Front(); e != nil; e = e.Next() {
		t := e.Value.(*task)
		if t.key == key {
			slot.Remove(e)
			// 已摘除，cancelled 可清；但保留也无害（scheduleIn 会自清）。
			delete(tw.cancelled, key)
			return
		}
	}
}

// exec 执行任务回调，带 panic recover，避免单个 job 崩溃影响整个调度线程。
func (tw *TimeWheel) exec(t *task) {
	if tw.asyncJob {
		go func(data interface{}) {
			defer tw.recoverJob()
			tw.job(data)
		}(t.data)
		return
	}
	defer tw.recoverJob()
	tw.job(t.data)
}

func (tw *TimeWheel) recoverJob() {
	// 这里不 import logger（避免 util 层反向依赖 api 层）；panic 静默恢复，保证
	// 调度线程存活。如需上报，调用方可在 job 内自行 recover+log。
	recover()
}
