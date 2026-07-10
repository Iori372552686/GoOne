// Package pprofcollect 从被测 Go 服务器的 pprof HTTP 端点采集资源指标与 profile 存档。
//
//   - 实时指标（每 sample interval）：协程数、heap inuse、CPU 核占用估算
//   - 定时存档（每 profile interval）：heap / goroutine / cpu profile 落盘，
//     可用 go tool pprof 单独查看
//
// 服务器 pprof 不可达时全部降级为 nil 指标，不影响压测主流程。
package pprofcollect

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
)

// Collector pprof 指标采集器。
type Collector struct {
	baseURL    string // http://host:port
	client     *http.Client
	profileDir string // profile 存档目录；为空表示不存档

	mu      sync.RWMutex
	latest  *stats.ServerMetrics
	healthy atomic.Bool

	cpuBusy atomic.Bool // CPU 采样进行中（避免叠加）
}

func New(baseURL, profileDir string) *Collector {
	return &Collector{
		baseURL:    baseURL,
		profileDir: profileDir,
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Healthy 报告最近一次采集是否成功。
func (c *Collector) Healthy() bool { return c.healthy.Load() }

// Latest 返回最近一次成功采集的指标；从未成功时返回 nil。
func (c *Collector) Latest() *stats.ServerMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.latest == nil {
		return nil
	}
	m := *c.latest
	return &m
}

// Run 启动采集循环，阻塞直到 ctx 取消。
//   - sampleInterval：实时指标采集间隔
//   - profileInterval：profile 存档间隔；<=0 关闭存档
func (c *Collector) Run(ctx context.Context, sampleInterval, profileInterval time.Duration) {
	if sampleInterval <= 0 {
		sampleInterval = 5 * time.Second
	}

	sampleTicker := time.NewTicker(sampleInterval)
	defer sampleTicker.Stop()

	var profileCh <-chan time.Time
	if profileInterval > 0 && c.profileDir != "" {
		profileTicker := time.NewTicker(profileInterval)
		defer profileTicker.Stop()
		profileCh = profileTicker.C
	}

	// 启动即采一次
	c.sample(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sampleTicker.C:
			c.sample(ctx)
		case <-profileCh:
			go c.saveProfiles(ctx)
		}
	}
}

// ---------------------------------------------------------------------------
// 实时指标
// ---------------------------------------------------------------------------

func (c *Collector) sample(ctx context.Context) {
	m := &stats.ServerMetrics{CPUCores: -1, HeapBytes: -1, Goroutines: -1}
	ok := false

	if n, err := c.goroutineCount(ctx); err == nil {
		m.Goroutines = n
		ok = true
	}
	if bytes, err := c.heapInuse(ctx); err == nil {
		m.HeapBytes = bytes
		ok = true
	}

	c.healthy.Store(ok)
	if !ok {
		return
	}

	// CPU 采样代价高（阻塞 profile 窗口），异步进行且不叠加；沿用上次值
	c.mu.RLock()
	if c.latest != nil {
		m.CPUCores = c.latest.CPUCores
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.latest = m
	c.mu.Unlock()

	if c.cpuBusy.CompareAndSwap(false, true) {
		go func() {
			defer c.cpuBusy.Store(false)
			if cores, err := c.cpuCores(ctx, 5*time.Second); err == nil {
				c.mu.Lock()
				if c.latest != nil {
					c.latest.CPUCores = cores
				}
				c.mu.Unlock()
			}
		}()
	}
}

var goroutineTotalRe = regexp.MustCompile(`^goroutine profile: total (\d+)`)

// goroutineCount 解析 /debug/pprof/goroutine?debug=1 首行 "goroutine profile: total N"。
func (c *Collector) goroutineCount(ctx context.Context) (int64, error) {
	resp, err := c.get(ctx, "/debug/pprof/goroutine?debug=1")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	if scanner.Scan() {
		if m := goroutineTotalRe.FindStringSubmatch(scanner.Text()); m != nil {
			return strconv.ParseInt(m[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("unexpected goroutine profile header")
}

var heapInuseRe = regexp.MustCompile(`# HeapInuse = (\d+)`)

// heapInuse 解析 /debug/pprof/heap?debug=1 尾部的 "# HeapInuse = N" 运行时统计行。
func (c *Collector) heapInuse(ctx context.Context) (int64, error) {
	resp, err := c.get(ctx, "/debug/pprof/heap?debug=1")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if m := heapInuseRe.FindStringSubmatch(scanner.Text()); m != nil {
			return strconv.ParseInt(m[1], 10, 64)
		}
	}
	return 0, fmt.Errorf("HeapInuse not found in heap profile")
}

// cpuCores 拉取 seconds 窗口的 CPU profile，用 "样本 CPU 总时长 / 窗口时长" 估算核占用。
// Go CPU profile 采样频率固定 100Hz，每个样本代表 10ms CPU 时间；
// 样本数通过轻量 protobuf wire 扫描统计（见 countCPUSamples），无需引入 pprof 解析依赖。
func (c *Collector) cpuCores(ctx context.Context, window time.Duration) (float64, error) {
	sec := int(window.Seconds())
	if sec < 1 {
		sec = 1
	}
	resp, err := c.get(ctx, fmt.Sprintf("/debug/pprof/profile?seconds=%d", sec))
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, err
	}
	samples, err := countCPUSamples(data)
	if err != nil {
		return -1, err
	}
	// 100Hz 采样：每个样本 10ms CPU 时间
	cpuSeconds := float64(samples) * 0.01
	return cpuSeconds / float64(sec), nil
}

// ---------------------------------------------------------------------------
// profile 存档
// ---------------------------------------------------------------------------

// saveProfiles 拉取 heap/goroutine/cpu profile 并落盘：
// {profileDir}/{timestamp}/heap.pb.gz 等。
func (c *Collector) saveProfiles(ctx context.Context) {
	stamp := time.Now().Format("20060102_150405")
	dir := filepath.Join(c.profileDir, stamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	c.download(ctx, "/debug/pprof/heap", filepath.Join(dir, "heap.pb.gz"))
	c.download(ctx, "/debug/pprof/goroutine", filepath.Join(dir, "goroutine.pb.gz"))
	c.download(ctx, "/debug/pprof/profile?seconds=10", filepath.Join(dir, "cpu.pb.gz"))
}

func (c *Collector) download(ctx context.Context, path, dest string) {
	// CPU profile 需要等待窗口时间，用更长超时
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	f, err := os.Create(dest)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.Copy(f, resp.Body)
}

func (c *Collector) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}
	return resp, nil
}
