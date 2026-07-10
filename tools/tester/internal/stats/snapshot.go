package stats

import (
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// 快照（面板与报告的只读视图）
// ---------------------------------------------------------------------------

// ProtoSnapshot 单个协议的统计快照。
type ProtoSnapshot struct {
	Key    ProtoKey
	Module string
	Name   string

	Total    int64
	Success  int64
	Timeout  int64
	SendFail int64
	BizFail  int64

	Min time.Duration
	Max time.Duration
	Avg time.Duration
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration

	// Buckets[i] 为第 i 个延迟桶计数，标签见 BucketLabel(i)。
	Buckets [numBuckets]int64
	// ErrCodes 业务错误码 -> 次数。
	ErrCodes map[int32]int64
}

// SuccessRate 协议通过率（0-1）；无请求时返回 1。
func (p *ProtoSnapshot) SuccessRate() float64 {
	if p.Total == 0 {
		return 1
	}
	return float64(p.Success) / float64(p.Total)
}

// BucketRatio 第 i 个延迟桶占比（0-1，分母为有 RTT 的请求数）。
func (p *ProtoSnapshot) BucketRatio(i int) float64 {
	var withRTT int64
	for _, b := range p.Buckets {
		withRTT += b
	}
	if withRTT == 0 {
		return 0
	}
	return float64(p.Buckets[i]) / float64(withRTT)
}

// ModuleSnapshot 单个业务模块的统计快照。
type ModuleSnapshot struct {
	Name    string
	Loops   int64
	LoopsOK int64
	AvgLoop time.Duration
	TPS     float64 // 全程平均：完成循环数 / 运行时长
	Protos  []*ProtoSnapshot
}

func (m *ModuleSnapshot) PassRate() float64 {
	if m.Loops == 0 {
		return 1
	}
	return float64(m.LoopsOK) / float64(m.Loops)
}

// Snapshot 一次完整快照。
type Snapshot struct {
	TakenAt time.Time
	Elapsed time.Duration

	TotalRequests int64
	TotalLoops    int64
	TotalErrors   int64
	Online        int64
	AvgQPS        float64 // 全程平均
	AvgTPS        float64 // 全程平均

	Modules []*ModuleSnapshot // 按名称排序
	Samples []Sample          // 时间序列（copy）
	Errors  []ErrorSample     // 错误采样（copy）
}

// Snapshot 生成当前统计的只读快照。
func (c *Collector) Snapshot() *Snapshot {
	now := time.Now()
	elapsed := now.Sub(c.startAt)

	c.mu.RLock()
	protoKeys := make([]ProtoKey, 0, len(c.protos))
	for k := range c.protos {
		protoKeys = append(protoKeys, k)
	}
	moduleNames := make([]string, 0, len(c.modules))
	for name := range c.modules {
		moduleNames = append(moduleNames, name)
	}
	c.mu.RUnlock()

	// 协议快照按模块分组
	byModule := make(map[string][]*ProtoSnapshot)
	for _, key := range protoKeys {
		c.mu.RLock()
		p := c.protos[key]
		c.mu.RUnlock()
		ps := snapshotProto(key, p)
		byModule[p.module] = append(byModule[p.module], ps)
	}

	// 模块快照（含只有协议没有循环记录的模块，如 "core"）
	nameSet := make(map[string]struct{}, len(moduleNames))
	for _, n := range moduleNames {
		nameSet[n] = struct{}{}
	}
	for n := range byModule {
		nameSet[n] = struct{}{}
	}

	modules := make([]*ModuleSnapshot, 0, len(nameSet))
	for name := range nameSet {
		ms := &ModuleSnapshot{Name: name, Protos: byModule[name]}
		sort.Slice(ms.Protos, func(i, j int) bool {
			return ms.Protos[i].Key.Cmd < ms.Protos[j].Key.Cmd
		})

		c.mu.RLock()
		m := c.modules[name]
		c.mu.RUnlock()
		if m != nil {
			ms.Loops = m.loops.Load()
			ms.LoopsOK = m.loopsOK.Load()
			if ms.Loops > 0 {
				ms.AvgLoop = time.Duration(m.loopSumNs.Load() / ms.Loops)
			}
			if elapsed > 0 {
				ms.TPS = float64(ms.LoopsOK) / elapsed.Seconds()
			}
		}
		modules = append(modules, ms)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })

	c.sampleMu.Lock()
	samples := make([]Sample, len(c.samples))
	copy(samples, c.samples)
	c.sampleMu.Unlock()

	c.errSampleMu.Lock()
	errs := make([]ErrorSample, len(c.errSamples))
	copy(errs, c.errSamples)
	c.errSampleMu.Unlock()

	snap := &Snapshot{
		TakenAt:       now,
		Elapsed:       elapsed,
		TotalRequests: c.totalReq.Load(),
		TotalLoops:    c.totalLoops.Load(),
		TotalErrors:   c.totalErrors.Load(),
		Online:        c.online.Load(),
		Modules:       modules,
		Samples:       samples,
		Errors:        errs,
	}
	if elapsed > 0 {
		snap.AvgQPS = float64(snap.TotalRequests) / elapsed.Seconds()
		snap.AvgTPS = float64(snap.TotalLoops) / elapsed.Seconds()
	}
	return snap
}

func snapshotProto(key ProtoKey, p *protoStat) *ProtoSnapshot {
	ps := &ProtoSnapshot{
		Key:      key,
		Module:   p.module,
		Name:     p.name,
		Total:    p.total.Load(),
		Success:  p.success.Load(),
		Timeout:  p.timeout.Load(),
		SendFail: p.sendFail.Load(),
		BizFail:  p.bizFail.Load(),
		ErrCodes: make(map[int32]int64),
	}

	var withRTT int64
	for i := range ps.Buckets {
		ps.Buckets[i] = p.buckets[i].Load()
		withRTT += ps.Buckets[i]
	}

	if withRTT > 0 {
		ps.Avg = time.Duration(p.sumNs.Load() / withRTT)
		ps.Min = time.Duration(p.minNs.Load())
		ps.Max = time.Duration(p.maxNs.Load())
		ps.P50 = percentileFromBuckets(ps.Buckets, withRTT, 0.50, ps.Min, ps.Max)
		ps.P95 = percentileFromBuckets(ps.Buckets, withRTT, 0.95, ps.Min, ps.Max)
		ps.P99 = percentileFromBuckets(ps.Buckets, withRTT, 0.99, ps.Min, ps.Max)
	}

	p.errMu.Lock()
	for code, n := range p.errCodes {
		ps.ErrCodes[code] = n
	}
	p.errMu.Unlock()

	return ps
}

// percentileFromBuckets 由直方图桶线性插值估算分位数。
// 桶内假设均匀分布；首桶下界取 min，尾桶上界取 max。
func percentileFromBuckets(buckets [numBuckets]int64, total int64, q float64, min, max time.Duration) time.Duration {
	target := int64(float64(total)*q + 0.5)
	if target < 1 {
		target = 1
	}

	var cum int64
	for i, n := range buckets {
		if n == 0 {
			continue
		}
		if cum+n >= target {
			lo, hi := bucketBounds(i, min, max)
			frac := float64(target-cum) / float64(n)
			return lo + time.Duration(frac*float64(hi-lo))
		}
		cum += n
	}
	return max
}

func bucketBounds(i int, min, max time.Duration) (time.Duration, time.Duration) {
	var lo, hi time.Duration
	if i == 0 {
		lo = 0
	} else {
		lo = time.Duration(latencyBucketsMs[i-1]) * time.Millisecond
	}
	if i >= len(latencyBucketsMs) {
		hi = max
		if hi < lo {
			hi = lo
		}
	} else {
		hi = time.Duration(latencyBucketsMs[i]) * time.Millisecond
	}
	if lo < min && i == 0 {
		lo = min
	}
	return lo, hi
}
