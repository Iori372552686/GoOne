// Package report 压力测试 Markdown 报告生成。
//
// 报告结构（行业惯例）：概要 → 全局指标（含时间序列图表）→ 服务器资源 →
// 各业务模块（每协议独立小节：延迟分位数、延迟区间占比、错误码分布）→ 错误明细。
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
)

// Meta 报告头部元信息。
type Meta struct {
	Title      string
	StopReason string // 到时结束 / 手动停止 / 中断（Ctrl+C）
	Mode       string // random / sequential
	Loop       bool
	Players    int   // 目标玩家数（结束时刻）
	StartUID   int64 // 压测 UID 段起点
	GatewayURL string
	PprofURL   string
	ProfileDir string // profile 存档目录；空表示未开启
	Modules    []string
}

// Write 生成报告文件，返回 Markdown 文件路径。
// 同时输出同名的 JSON 原始数据文件（便于程序化分析与 benchstat 对比）。
func Write(dir string, meta Meta, snap *stats.Snapshot) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}
	stamp := snap.TakenAt.Format("20060102_150405")
	name := fmt.Sprintf("stress_%s.md", stamp)
	path := filepath.Join(dir, name)

	var b strings.Builder
	render(&b, meta, snap)

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}

	// 输出 JSON 原始数据（与 Markdown 同名，.json 扩展名）。
	if err := writeJSON(dir, stamp, meta, snap); err != nil {
		// JSON 失败不阻断 Markdown 报告（非关键路径）。
		fmt.Fprintf(os.Stderr, "warn: write json: %v\n", err)
	}
	return path, nil
}

// writeJSON 把报告元信息与快照序列化为 JSON 文件，供程序化分析。
func writeJSON(dir, stamp string, meta Meta, snap *stats.Snapshot) error {
	type reportData struct {
		Meta     Meta            `json:"meta"`
		Snapshot *stats.Snapshot `json:"snapshot"`
	}
	data := reportData{Meta: meta, Snapshot: snap}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("stress_%s.json", stamp))
	return os.WriteFile(path, raw, 0o644)
}

func render(b *strings.Builder, meta Meta, snap *stats.Snapshot) {
	title := meta.Title
	if title == "" {
		title = "全流程压力测试报告"
	}
	fmt.Fprintf(b, "# %s\n\n", title)

	renderSummary(b, meta, snap)
	renderGlobal(b, snap)
	renderServer(b, meta, snap)
	renderModules(b, snap)
	renderErrors(b, snap)
}

// ---------------------------------------------------------------------------
// 一、测试概要
// ---------------------------------------------------------------------------

func renderSummary(b *strings.Builder, meta Meta, snap *stats.Snapshot) {
	b.WriteString("## 一、测试概要\n\n")
	b.WriteString("| 项 | 值 |\n|---|---|\n")
	fmt.Fprintf(b, "| 结束时间 | %s |\n", snap.TakenAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "| 运行时长 | %s |\n", fmtDuration(snap.Elapsed))
	fmt.Fprintf(b, "| 结束原因 | %s |\n", meta.StopReason)
	fmt.Fprintf(b, "| 压测模式 | %s（loop=%v） |\n", meta.Mode, meta.Loop)
	fmt.Fprintf(b, "| 目标玩家数 | %d |\n", meta.Players)
	fmt.Fprintf(b, "| UID 段 | %d ~ %d |\n", meta.StartUID, meta.StartUID+int64(meta.Players)-1)
	fmt.Fprintf(b, "| 网关 | %s |\n", meta.GatewayURL)
	fmt.Fprintf(b, "| 覆盖模块 | %s |\n", strings.Join(meta.Modules, ", "))
	fmt.Fprintf(b, "| 总请求数 | %d |\n", snap.TotalRequests)
	fmt.Fprintf(b, "| 总业务循环数 | %d |\n", snap.TotalLoops)
	fmt.Fprintf(b, "| 总错误数 | %d |\n", snap.TotalErrors)

	var succ float64 = 100
	if snap.TotalRequests > 0 {
		succ = float64(snap.TotalRequests-snap.TotalErrors) / float64(snap.TotalRequests) * 100
	}
	fmt.Fprintf(b, "| 总体成功率 | %.2f%% |\n", succ)
	b.WriteString("\n")
}

// ---------------------------------------------------------------------------
// 二、全局指标
// ---------------------------------------------------------------------------

func renderGlobal(b *strings.Builder, snap *stats.Snapshot) {
	b.WriteString("## 二、全局指标\n\n")
	b.WriteString("| 指标 | 值 |\n|---|---|\n")
	fmt.Fprintf(b, "| 平均 QPS（协议请求/秒） | %.1f |\n", snap.AvgQPS)
	fmt.Fprintf(b, "| 平均 TPS（业务循环/秒） | %.1f |\n", snap.AvgTPS)

	var peakQPS, peakTPS float64
	var peakOnline int64
	for _, s := range snap.Samples {
		if s.QPS > peakQPS {
			peakQPS = s.QPS
		}
		if s.TPS > peakTPS {
			peakTPS = s.TPS
		}
		if s.Online > peakOnline {
			peakOnline = s.Online
		}
	}
	fmt.Fprintf(b, "| 峰值 QPS | %.1f |\n", peakQPS)
	fmt.Fprintf(b, "| 峰值 TPS | %.1f |\n", peakTPS)
	fmt.Fprintf(b, "| 峰值在线 | %d |\n", peakOnline)
	b.WriteString("\n")

	if len(snap.Samples) >= 2 {
		samples := downsample(snap.Samples, 30)

		b.WriteString("### 在线玩家数 / QPS / TPS 走势\n\n")
		writeXYChart(b, "在线玩家数", samples, func(s stats.Sample) float64 { return float64(s.Online) })
		writeXYChart(b, "QPS", samples, func(s stats.Sample) float64 { return s.QPS })
		writeXYChart(b, "TPS", samples, func(s stats.Sample) float64 { return s.TPS })

		b.WriteString("### 采样明细\n\n")
		b.WriteString("| 时间 | 在线 | QPS | TPS | CPU(核) | 内存 | 协程数 |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, s := range samples {
			fmt.Fprintf(b, "| %s | %d | %.1f | %.1f | %s | %s | %s |\n",
				s.At.Format("15:04:05"), s.Online, s.QPS, s.TPS,
				fmtCPU(s.CPUCores), fmtBytes(s.HeapBytes), fmtCount(s.Goroutines))
		}
		b.WriteString("\n")
	}
}

// ---------------------------------------------------------------------------
// 三、服务器资源（pprof）
// ---------------------------------------------------------------------------

func renderServer(b *strings.Builder, meta Meta, snap *stats.Snapshot) {
	b.WriteString("## 三、服务器资源（pprof）\n\n")

	valid := make([]stats.Sample, 0, len(snap.Samples))
	for _, s := range snap.Samples {
		if s.Goroutines >= 0 || s.HeapBytes >= 0 || s.CPUCores >= 0 {
			valid = append(valid, s)
		}
	}
	if len(valid) == 0 {
		fmt.Fprintf(b, "未采集到服务器 pprof 指标（%s 不可达或未开启）。\n\n", meta.PprofURL)
		return
	}

	samples := downsample(valid, 30)
	writeXYChart(b, "CPU 核占用", samples, func(s stats.Sample) float64 {
		if s.CPUCores < 0 {
			return 0
		}
		return s.CPUCores
	})
	writeXYChart(b, "Heap 内存 (MB)", samples, func(s stats.Sample) float64 {
		if s.HeapBytes < 0 {
			return 0
		}
		return float64(s.HeapBytes) / (1 << 20)
	})
	writeXYChart(b, "协程数", samples, func(s stats.Sample) float64 {
		if s.Goroutines < 0 {
			return 0
		}
		return float64(s.Goroutines)
	})

	if meta.ProfileDir != "" {
		fmt.Fprintf(b, "### Profile 存档\n\n目录：`%s`（heap.pb.gz / goroutine.pb.gz / cpu.pb.gz，按时间戳分目录）\n\n", meta.ProfileDir)
		b.WriteString("查看方式：`go tool pprof <文件>`\n\n")
	}
}

// ---------------------------------------------------------------------------
// 四、业务模块明细
// ---------------------------------------------------------------------------

func renderModules(b *strings.Builder, snap *stats.Snapshot) {
	b.WriteString("## 四、业务模块明细\n\n")

	for _, m := range snap.Modules {
		fmt.Fprintf(b, "### 模块 %s\n\n", m.Name)

		if m.Loops > 0 {
			b.WriteString("| 指标 | 值 |\n|---|---|\n")
			fmt.Fprintf(b, "| 循环次数 | %d |\n", m.Loops)
			fmt.Fprintf(b, "| 循环通过率 | %.2f%% |\n", m.PassRate()*100)
			fmt.Fprintf(b, "| 平均单轮耗时 | %s |\n", fmtDuration(m.AvgLoop))
			fmt.Fprintf(b, "| 模块 TPS | %.2f |\n", m.TPS)
			b.WriteString("\n")
		}

		if len(m.Protos) == 0 {
			b.WriteString("（无协议统计）\n\n")
			continue
		}

		// 协议总览
		b.WriteString("| 协议 | cmd | 请求数 | 成功率 | Avg | P50 | P95 | P99 | Min | Max |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
		for _, p := range m.Protos {
			fmt.Fprintf(b, "| %s | 0x%x | %d | %.2f%% | %s | %s | %s | %s | %s | %s |\n",
				protoDisplayName(p), p.Key.Cmd, p.Total, p.SuccessRate()*100,
				fmtDuration(p.Avg), fmtDuration(p.P50), fmtDuration(p.P95), fmtDuration(p.P99),
				fmtDuration(p.Min), fmtDuration(p.Max))
		}
		b.WriteString("\n")

		// 每协议独立小节：延迟区间占比 + 错误码
		for _, p := range m.Protos {
			fmt.Fprintf(b, "#### %s（cmd=0x%x）\n\n", protoDisplayName(p), p.Key.Cmd)
			fmt.Fprintf(b, "请求 %d 次：成功 %d，业务错误 %d，超时 %d，发送失败 %d\n\n",
				p.Total, p.Success, p.BizFail, p.Timeout, p.SendFail)

			// 延迟区间占比（只列非零桶）
			b.WriteString("| 延迟区间 | 次数 | 占比 |\n|---|---|---|\n")
			for i, n := range p.Buckets {
				if n == 0 {
					continue
				}
				fmt.Fprintf(b, "| %s | %d | %.1f%% |\n", stats.BucketLabel(i), n, p.BucketRatio(i)*100)
			}
			b.WriteString("\n")

			if len(p.ErrCodes) > 0 {
				b.WriteString("| 错误码 | 次数 |\n|---|---|\n")
				codes := make([]int32, 0, len(p.ErrCodes))
				for code := range p.ErrCodes {
					codes = append(codes, code)
				}
				sort.Slice(codes, func(i, j int) bool { return p.ErrCodes[codes[i]] > p.ErrCodes[codes[j]] })
				for _, code := range codes {
					fmt.Fprintf(b, "| %d | %d |\n", code, p.ErrCodes[code])
				}
				b.WriteString("\n")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 五、错误明细
// ---------------------------------------------------------------------------

func renderErrors(b *strings.Builder, snap *stats.Snapshot) {
	b.WriteString("## 五、错误采样\n\n")
	if len(snap.Errors) == 0 {
		b.WriteString("无错误采样。\n")
		return
	}
	b.WriteString("| 时间 | 模块 | 协议 | 详情 |\n|---|---|---|---|\n")
	for _, e := range snap.Errors {
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n",
			e.At.Format("15:04:05"), e.Module, e.Proto, sanitizeCell(e.Detail))
	}
	b.WriteString("\n")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func protoDisplayName(p *stats.ProtoSnapshot) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Key.String()
}

// writeXYChart 输出 mermaid xychart-beta 折线图。
func writeXYChart(b *strings.Builder, title string, samples []stats.Sample, y func(stats.Sample) float64) {
	if len(samples) < 2 {
		return
	}
	b.WriteString("```mermaid\nxychart-beta\n")
	fmt.Fprintf(b, "    title \"%s\"\n", title)

	labels := make([]string, len(samples))
	values := make([]string, len(samples))
	maxV := 0.0
	for i, s := range samples {
		labels[i] = s.At.Format("15:04:05")
		v := y(s)
		values[i] = fmt.Sprintf("%.1f", v)
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	fmt.Fprintf(b, "    x-axis [%s]\n", strings.Join(labels, ", "))
	fmt.Fprintf(b, "    y-axis \"%s\" 0 --> %.1f\n", title, maxV*1.1)
	fmt.Fprintf(b, "    line [%s]\n", strings.Join(values, ", "))
	b.WriteString("```\n\n")
}

// downsample 均匀抽稀到不超过 max 个点（图表可读性）。
func downsample(samples []stats.Sample, max int) []stats.Sample {
	if len(samples) <= max {
		return samples
	}
	out := make([]stats.Sample, 0, max)
	step := float64(len(samples)-1) / float64(max-1)
	for i := 0; i < max; i++ {
		out = append(out, samples[int(float64(i)*step+0.5)])
	}
	return out
}

func fmtDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "-"
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return d.Round(time.Second).String()
	}
}

func fmtBytes(n int64) string {
	switch {
	case n < 0:
		return "N/A"
	case n >= 1<<30:
		return fmt.Sprintf("%.2fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.0fKB", float64(n)/(1<<10))
	}
}

func fmtCPU(cores float64) string {
	if cores < 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.2f", cores)
}

func fmtCount(n int64) string {
	if n < 0 {
		return "N/A"
	}
	return fmt.Sprintf("%d", n)
}

func sanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
