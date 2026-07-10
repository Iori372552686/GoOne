// Package stats 压测/回归通用的内存统计中心。
//
// 三个层次：
//   - 协议级：按 cmd 独立统计 RTT 直方图、分位数、错误码分布
//   - 模块级：业务循环次数、通过率、模块 TPS
//   - 全局级：QPS、TPS、在线玩家数、时间序列采样
//
// 全部方法 goroutine 安全；热路径使用原子计数，仅在新协议/新错误码首次出现时加写锁。
package stats

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// latencyBucketsMs 固定延迟桶上界（毫秒），最后一个桶为 1000ms+。
var latencyBucketsMs = []int64{1, 5, 10, 20, 50, 100, 200, 500, 1000}

const numBuckets = 10 // len(latencyBucketsMs) + 1

// BucketLabel 返回第 i 个延迟桶的展示名，如 "1-5ms"、"1000ms+"。
func BucketLabel(i int) string {
	switch {
	case i == 0:
		return fmt.Sprintf("0-%dms", latencyBucketsMs[0])
	case i >= len(latencyBucketsMs):
		return fmt.Sprintf("%dms+", latencyBucketsMs[len(latencyBucketsMs)-1])
	default:
		return fmt.Sprintf("%d-%dms", latencyBucketsMs[i-1], latencyBucketsMs[i])
	}
}

func bucketIndex(rtt time.Duration) int {
	ms := rtt.Milliseconds()
	for i, ub := range latencyBucketsMs {
		if ms < ub {
			return i
		}
	}
	return len(latencyBucketsMs)
}

// ---------------------------------------------------------------------------
// 协议级统计
// ---------------------------------------------------------------------------

// ProtoKey 唯一标识一个协议（cmd）。
type ProtoKey struct {
	Cmd uint32
}

func (k ProtoKey) String() string { return fmt.Sprintf("cmd_0x%x", k.Cmd) }

type protoStat struct {
	module string
	name   string // 协议展示名，如 "login.LoginReq"，可为空

	total    atomic.Int64
	success  atomic.Int64
	timeout  atomic.Int64
	sendFail atomic.Int64
	bizFail  atomic.Int64 // 业务错误码非 0

	sumNs atomic.Int64
	minNs atomic.Int64
	maxNs atomic.Int64

	buckets [numBuckets]atomic.Int64

	errMu    sync.Mutex
	errCodes map[int32]int64
}

func newProtoStat(module, name string) *protoStat {
	p := &protoStat{module: module, name: name, errCodes: make(map[int32]int64)}
	p.minNs.Store(int64(1<<63 - 1))
	return p
}

// Outcome 一次协议请求的结果分类。
type Outcome int

const (
	OutcomeSuccess  Outcome = iota // 收到响应且业务错误码为 0
	OutcomeBizError                // 收到响应但业务错误码非 0
	OutcomeTimeout                 // 等待响应超时
	OutcomeSendFail                // 发送失败（连接断开等）
)

// ---------------------------------------------------------------------------
// 模块级统计
// ---------------------------------------------------------------------------

type moduleStat struct {
	loops     atomic.Int64
	loopsOK   atomic.Int64
	loopSumNs atomic.Int64
}

// ---------------------------------------------------------------------------
// Collector
// ---------------------------------------------------------------------------

// Collector 一次测试运行的统计实例。
type Collector struct {
	startAt time.Time

	mu      sync.RWMutex
	protos  map[ProtoKey]*protoStat
	modules map[string]*moduleStat

	totalReq    atomic.Int64
	totalLoops  atomic.Int64
	totalErrors atomic.Int64
	online      atomic.Int64

	// 上一次采样点，用于计算区间 QPS/TPS
	lastSampleAt    time.Time
	lastSampleReq   int64
	lastSampleLoops int64
	sampleMu        sync.Mutex
	samples         []Sample

	errSampleMu sync.Mutex
	errSamples  []ErrorSample
}

// Sample 时间序列采样点（供面板与报告图表使用）。
type Sample struct {
	At     time.Time
	Online int64
	QPS    float64 // 采样区间内每秒协议请求数
	TPS    float64 // 采样区间内每秒完成业务循环数

	// 服务器资源指标（由 pprof 采集器写入；采集失败时为 -1）
	CPUCores   float64 // 估算 CPU 核占用
	HeapBytes  int64   // heap inuse
	Goroutines int64
}

// ErrorSample 错误采样（保留少量典型错误用于报告展示）。
type ErrorSample struct {
	At     time.Time
	Module string
	Proto  string
	Detail string
}

const maxErrorSamples = 100

func NewCollector() *Collector {
	now := time.Now()
	return &Collector{
		startAt:      now,
		protos:       make(map[ProtoKey]*protoStat),
		modules:      make(map[string]*moduleStat),
		lastSampleAt: now,
	}
}

func (c *Collector) StartAt() time.Time { return c.startAt }

func (c *Collector) proto(key ProtoKey, module, name string) *protoStat {
	c.mu.RLock()
	p, ok := c.protos[key]
	c.mu.RUnlock()
	if ok {
		return p
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok = c.protos[key]; ok {
		return p
	}
	p = newProtoStat(module, name)
	c.protos[key] = p
	return p
}

func (c *Collector) module(name string) *moduleStat {
	c.mu.RLock()
	m, ok := c.modules[name]
	c.mu.RUnlock()
	if ok {
		return m
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok = c.modules[name]; ok {
		return m
	}
	m = &moduleStat{}
	c.modules[name] = m
	return m
}

// RecordRequest 记录一次协议请求结果。
//   - module：发起请求的业务模块名（登录等基础流程用 "core"）
//   - name：协议展示名，可为空
//   - rtt：请求-响应往返延迟；OutcomeSendFail 时忽略
//   - errCode：业务错误码（Outcome 为 OutcomeBizError 时有效）
func (c *Collector) RecordRequest(key ProtoKey, module, name string, rtt time.Duration, outcome Outcome, errCode int32) {
	p := c.proto(key, module, name)
	p.total.Add(1)
	c.totalReq.Add(1)

	switch outcome {
	case OutcomeSuccess:
		p.success.Add(1)
	case OutcomeBizError:
		p.bizFail.Add(1)
		c.totalErrors.Add(1)
		p.errMu.Lock()
		p.errCodes[errCode]++
		p.errMu.Unlock()
	case OutcomeTimeout:
		p.timeout.Add(1)
		c.totalErrors.Add(1)
	case OutcomeSendFail:
		p.sendFail.Add(1)
		c.totalErrors.Add(1)
		return // 无有效 RTT
	}

	ns := rtt.Nanoseconds()
	p.sumNs.Add(ns)
	for {
		old := p.minNs.Load()
		if ns >= old || p.minNs.CompareAndSwap(old, ns) {
			break
		}
	}
	for {
		old := p.maxNs.Load()
		if ns <= old || p.maxNs.CompareAndSwap(old, ns) {
			break
		}
	}
	p.buckets[bucketIndex(rtt)].Add(1)
}

// RecordLoop 记录一次业务循环（一轮 StressRunner 或一轮回归用例集）。
func (c *Collector) RecordLoop(module string, ok bool, elapsed time.Duration) {
	m := c.module(module)
	m.loops.Add(1)
	m.loopSumNs.Add(elapsed.Nanoseconds())
	if ok {
		m.loopsOK.Add(1)
	}
	c.totalLoops.Add(1)
}

// RecordError 保留典型错误样本（最多 maxErrorSamples 条）。
func (c *Collector) RecordError(module, protoName, detail string) {
	c.errSampleMu.Lock()
	defer c.errSampleMu.Unlock()
	if len(c.errSamples) >= maxErrorSamples {
		return
	}
	c.errSamples = append(c.errSamples, ErrorSample{
		At: time.Now(), Module: module, Proto: protoName, Detail: detail,
	})
}

// SetOnline 更新实时在线玩家数。
func (c *Collector) SetOnline(n int64) { c.online.Store(n) }

func (c *Collector) AddOnline(delta int64) { c.online.Add(delta) }

func (c *Collector) Online() int64 { return c.online.Load() }

// TakeSample 采样一个时间序列点；serverMetrics 可为 nil（表示 pprof 不可用）。
func (c *Collector) TakeSample(server *ServerMetrics) Sample {
	c.sampleMu.Lock()
	defer c.sampleMu.Unlock()

	now := time.Now()
	req := c.totalReq.Load()
	loops := c.totalLoops.Load()

	interval := now.Sub(c.lastSampleAt).Seconds()
	var qps, tps float64
	if interval > 0 {
		qps = float64(req-c.lastSampleReq) / interval
		tps = float64(loops-c.lastSampleLoops) / interval
	}
	c.lastSampleAt = now
	c.lastSampleReq = req
	c.lastSampleLoops = loops

	s := Sample{
		At: now, Online: c.online.Load(), QPS: qps, TPS: tps,
		CPUCores: -1, HeapBytes: -1, Goroutines: -1,
	}
	if server != nil {
		s.CPUCores = server.CPUCores
		s.HeapBytes = server.HeapBytes
		s.Goroutines = server.Goroutines
	}
	c.samples = append(c.samples, s)
	return s
}

// ServerMetrics pprof 采集到的服务器资源指标。
type ServerMetrics struct {
	CPUCores   float64
	HeapBytes  int64
	Goroutines int64
}
