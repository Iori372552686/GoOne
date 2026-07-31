// Package report capacity_matrix 把多份 stress JSON 原始报告汇总为容量矩阵 Markdown
// （V4 P1-03）。
//
// 用途：stress 是唯一容量主路径（capacity 工具已 Deprecated）。每次 C1~C4 阶梯跑完
// 产出一份 stress_<stamp>.json；本工具读取指定目录下全部 stress_*.json，按目标连接数
// 归类为 C1~C4 行，校验 SLO 通过条件，输出 docs/benchmarks/capacity-matrix.md。
//
// 设计为可测试的纯函数：ParseMatrix(dir) (*Matrix, error)；RenderMarkdown(*Matrix) string。
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

// 阶梯定义（与计划 P1-03 一致）。
type capacityTier struct {
	Name        string
	Connections int
	LoginRate   int // 登录/秒
	SteadyMsgPS int // 稳态消息/秒
}

// CapacityTiers 是 C1~C4 容量阶梯（V4 P1-03）。
var CapacityTiers = []capacityTier{
	{"C1", 1000, 100, 500},
	{"C2", 3000, 200, 1500},
	{"C3", 5000, 300, 2500},
	{"C4", 10000, 500, 5000},
}

// SLOTargets 是 C4 验收的量化目标（V4 P1-03 / observability_slo.md）。
type SLOTargets struct {
	ConnSuccessRate       float64 // 0.999
	BizSuccessRate        float64 // 0.999
	P99Latency            time.Duration
	CPUUsageFraction      float64 // 0.70
	RSSGrowthFraction     float64 // 0.05（最后 15 分钟）
	GCPauseP99            time.Duration
	ReadinessCloseSeconds float64
	DrainTimeoutSeconds   float64
}

// DefaultSLO 是计划 P1-03 的 C4 验收标准。
var DefaultSLO = SLOTargets{
	ConnSuccessRate:       0.999,
	BizSuccessRate:        0.999,
	P99Latency:            50 * time.Millisecond,
	CPUUsageFraction:      0.70,
	RSSGrowthFraction:     0.05,
	GCPauseP99:            20 * time.Millisecond,
	ReadinessCloseSeconds: 1,
	DrainTimeoutSeconds:   30,
}

// TierResult 是单个阶梯一次运行的汇总结果。
type TierResult struct {
	Tier        capacityTier
	Source      string // JSON 文件名
	OnPeak      int64  // 峰值在线
	TotalReq    int64
	TotalErr    int64
	SuccessRate float64
	P99         time.Duration
	SLO         SLOVerdict
}

// SLOVerdict 标记某阶梯是否通过 SLO，附失败原因。
type SLOVerdict struct {
	Pass    bool
	Reasons []string
}

// Matrix 是全部阶梯的汇总。
type Matrix struct {
	GeneratedAt time.Time
	Source      string // 汇总来源目录
	Tiers       []TierResult
}

// ParseMatrix 扫描 dir 下全部 stress_*.json，按峰值在线数匹配到最近阶梯，
// 返回汇总矩阵。无匹配文件时返回空矩阵与 nil error（调用方可据此判「未跑」）。
func ParseMatrix(dir string) (*Matrix, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "stress_*.json"))
	if err != nil {
		return nil, fmt.Errorf("glob stress reports: %w", err)
	}
	sort.Strings(entries)

	m := &Matrix{GeneratedAt: time.Now(), Source: dir}
	for _, f := range entries {
		tr, err := parseTierResult(f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filepath.Base(f), err)
		}
		if tr == nil {
			continue
		}
		m.Tiers = append(m.Tiers, *tr)
	}
	return m, nil
}

// parseTierResult 读取单份 JSON，匹配阶梯并计算 SLO。
func parseTierResult(path string) (*TierResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Meta     *jsonMeta       `json:"meta"`
		Snapshot *stats.Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Snapshot == nil {
		return nil, nil
	}

	tier := matchTier(doc.Snapshot.Online, doc.Meta.Players)
	if tier.Name == "" {
		return nil, nil // 不匹配任何阶梯（如预热跑），跳过。
	}

	totalReq := doc.Snapshot.TotalRequests
	totalErr := doc.Snapshot.TotalErrors
	successRate := 1.0
	if totalReq > 0 {
		successRate = float64(totalReq-totalErr) / float64(totalReq)
	}
	p99 := maxModuleP99(doc.Snapshot)

	return &TierResult{
		Tier:        tier,
		Source:      filepath.Base(path),
		OnPeak:      doc.Snapshot.Online,
		TotalReq:    totalReq,
		TotalErr:    totalErr,
		SuccessRate: successRate,
		P99:         p99,
		SLO:         judgeSLO(tier, successRate, p99, DefaultSLO),
	}, nil
}

// jsonMeta 是 report.Meta 的读取子集（仅需 Players 用于阶梯匹配）。
type jsonMeta struct {
	Players int    `json:"Players"`
	Title   string `json:"Title"`
}

// matchTier 按峰值在线数匹配最近阶梯（取 connections 上限不超过峰值的最大阶梯）。
func matchTier(peakOnline int64, metaPlayers int) capacityTier {
	peak := peakOnline
	if int64(metaPlayers) > peak {
		peak = int64(metaPlayers)
	}
	var best capacityTier
	for _, t := range CapacityTiers {
		if int64(t.Connections) <= peak {
			best = t
		}
	}
	return best
}

// maxModuleP99 取所有协议中最大的 P99 延迟。
func maxModuleP99(snap *stats.Snapshot) time.Duration {
	var max time.Duration
	for _, m := range snap.Modules {
		for _, p := range m.Protos {
			if p.P99 > max {
				max = p.P99
			}
		}
	}
	return max
}

// judgeSLO 根据成功率与 P99 判定阶梯是否通过（CPU/RSS/GC 需服务端指标，这里只校验
// 客户端可观测的两项；其余由 observability_slo.md 的服务端告警覆盖）。
func judgeSLO(tier capacityTier, successRate float64, p99 time.Duration, slo SLOTargets) SLOVerdict {
	v := SLOVerdict{Pass: true}
	if successRate < slo.BizSuccessRate {
		v.Pass = false
		v.Reasons = append(v.Reasons, fmt.Sprintf("成功率 %.4f < %.4f", successRate, slo.BizSuccessRate))
	}
	if p99 > 0 && p99 > slo.P99Latency {
		v.Pass = false
		v.Reasons = append(v.Reasons, fmt.Sprintf("P99 %v > %v", p99, slo.P99Latency))
	}
	return v
}

// RenderMarkdown 把矩阵渲染为容量矩阵 Markdown（docs/benchmarks/capacity-matrix.md）。
func RenderMarkdown(m *Matrix) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 容量矩阵（C1～C4）\n\n")
	fmt.Fprintf(&b, "> V4 P1-03：由 stress 唯一容量主路径产出（capacity 工具已 Deprecated）。\n")
	fmt.Fprintf(&b, "> 生成时间：%s　来源目录：`%s`\n\n", m.GeneratedAt.Format("2006-01-02 15:04:05"), m.Source)

	if len(m.Tiers) == 0 {
		b.WriteString("**暂无 stress 报告**：完成 C1～C4 阶梯压测后，将 `stress_*.json` 放入来源目录重新生成。\n\n")
	}

	b.WriteString("| 阶段 | 目标连接 | 峰值在线 | 总请求 | 错误数 | 成功率 | P99 | SLO |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|:---:|\n")
	for _, tr := range m.Tiers {
		verdict := "✅ 通过"
		if !tr.SLO.Pass {
			verdict = "❌ " + strings.Join(tr.SLO.Reasons, "; ")
		}
		if tr.TotalReq == 0 {
			verdict = "— 未跑"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %.4f | %v | %s |\n",
			tr.Tier.Name, tr.Tier.Connections, tr.OnPeak, tr.TotalReq, tr.TotalErr,
			tr.SuccessRate, tr.P99, verdict)
	}

	b.WriteString("\n## SLO 目标（C4 验收）\n\n")
	fmt.Fprintf(&b, "- 连接/业务成功率 ≥ %.3f\n", DefaultSLO.BizSuccessRate)
	fmt.Fprintf(&b, "- 框架链路 P99 ≤ %v\n", DefaultSLO.P99Latency)
	fmt.Fprintf(&b, "- CPU ≤ 分配核心的 %.0f%%\n", DefaultSLO.CPUUsageFraction*100)
	fmt.Fprintf(&b, "- 最后 15 分钟 RSS 增长 < %.0f%%\n", DefaultSLO.RSSGrowthFraction*100)
	fmt.Fprintf(&b, "- GC pause P99 ≤ %v\n", DefaultSLO.GCPauseP99)
	fmt.Fprintf(&b, "- readiness ≤ %.0fs 关闭，drain ≤ %.0fs 完成\n\n",
		DefaultSLO.ReadinessCloseSeconds, DefaultSLO.DrainTimeoutSeconds)

	b.WriteString("> CPU/RSS/GC/readiness/drain 需结合服务端 Prometheus 指标判定，见 `docs/observability_slo.md`。\n")
	return b.String()
}
