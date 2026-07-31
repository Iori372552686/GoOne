package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 写一份最小 stress JSON 到 tempdir，模拟 stress 报告输出结构。
func writeStressJSON(t *testing.T, dir, name string, online, players, totalReq, totalErr int64, p99Ms int) {
	t.Helper()
	doc := map[string]any{
		"meta": map[string]any{"Players": players, "Title": "stress"},
		"snapshot": map[string]any{
			"TakenAt":       "2026-07-31T12:00:00Z",
			"Elapsed":       1800000000000,
			"TotalRequests": totalReq,
			"TotalErrors":   totalErr,
			"Online":        online,
			"Modules": []map[string]any{
				{"Name": "login", "Protos": []map[string]any{
					{"Key": map[string]any{"Cmd": 1}, "P99": int64(time.Duration(p99Ms) * time.Millisecond)},
				}},
			},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestParseMatrixMatchesTierByPeakOnline 验证 P1-03：JSON 按峰值在线匹配到正确阶梯，
// SLO 成功率/P99 校验生效。
func TestParseMatrixMatchesTierByPeakOnline(t *testing.T) {
	dir := t.TempDir()
	// C1：1000 连接，成功率 100%，P99 20ms → 通过。
	writeStressJSON(t, dir, "stress_c1.json", 1000, 1000, 10000, 0, 20)
	// C3：5000 连接，成功率 99.8%（低于 99.9%），P99 60ms（超 50ms）→ 失败。
	writeStressJSON(t, dir, "stress_c3.json", 5000, 5000, 10000, 20, 60)

	m, err := ParseMatrix(dir)
	if err != nil {
		t.Fatalf("ParseMatrix: %v", err)
	}
	if len(m.Tiers) != 2 {
		t.Fatalf("expected 2 tier results, got %d", len(m.Tiers))
	}

	byName := map[string]TierResult{}
	for _, tr := range m.Tiers {
		byName[tr.Tier.Name] = tr
	}
	c1, ok := byName["C1"]
	if !ok {
		t.Fatal("C1 missing")
	}
	if !c1.SLO.Pass {
		t.Fatalf("C1 should pass SLO, got reasons %v", c1.SLO.Reasons)
	}
	c3, ok := byName["C3"]
	if !ok {
		t.Fatal("C3 missing")
	}
	if c3.SLO.Pass {
		t.Fatal("C3 should fail SLO (success rate + P99)")
	}
}

// TestParseMatrixEmptyDir 验证无报告时返回空矩阵（不报错）。
func TestParseMatrixEmptyDir(t *testing.T) {
	dir := t.TempDir()
	m, err := ParseMatrix(dir)
	if err != nil {
		t.Fatalf("ParseMatrix empty dir: %v", err)
	}
	if len(m.Tiers) != 0 {
		t.Fatalf("expected 0 tiers for empty dir, got %d", len(m.Tiers))
	}
}

// TestRenderMarkdownContainsTiers 验证 Markdown 渲染包含阶梯行与 SLO 目标。
func TestRenderMarkdownContainsTiers(t *testing.T) {
	m := &Matrix{
		GeneratedAt: time.Now(),
		Source:      "./tmp",
		Tiers: []TierResult{{
			Tier:        CapacityTiers[0],
			OnPeak:      1000,
			TotalReq:    5000,
			SuccessRate: 1.0,
			SLO:         SLOVerdict{Pass: true},
		}},
	}
	md := RenderMarkdown(m)
	for _, want := range []string{"C1", "容量矩阵", "SLO 目标", "✅ 通过"} {
		if !contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

// TestRenderMarkdownEmptyShowsPlaceholder 验证无数据时的占位提示。
func TestRenderMarkdownEmptyShowsPlaceholder(t *testing.T) {
	m := &Matrix{GeneratedAt: time.Now(), Source: "./tmp"}
	md := RenderMarkdown(m)
	if !contains(md, "暂无 stress 报告") {
		t.Errorf("empty matrix should show placeholder, got: %s", md)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
