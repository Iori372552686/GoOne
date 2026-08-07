package test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
	"github.com/xuri/excelize/v2"
)

// 本文件对 cfgtool 三项新功能的解析热点做 benchmark（黑盒端到端）。
// 通过构造不同规模的 xlsx 输入，跑完整 ParseFiles→GenProto→ParseProto→GenData 链路，
// 度量 ns/op 与 alloc/op，观察线性度。
//
// 外部 proto 在每个子 benchmark 的 b.ResetTimer 前重新加载一次（每轮 Clear 后需重载），
// 因此 IO 开销计入测量——这反映真实使用场景（每次跑 cfgtool 都要加载外部 proto）。

// repoRootB 是 repoRoot 的 *testing.B 版本（benchmark 无 *testing.T）。
func repoRootB(b *testing.B) string {
	b.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(thisFile, "..", "..", "..", ".."))
}

// buildBenchXlsx 构造一个含 N 行数据的 xlsx，返回路径与生成的 config 名。
// fieldType 决定测哪种解析热点：map / array / external。
func buildBenchXlsx(b *testing.B, dir string, n int, fieldType string) (string, string) {
	b.Helper()
	xlsxPath := filepath.Join(dir, fmt.Sprintf("bench_%s_%d.xlsx", fieldType, n))
	cfgName := fmt.Sprintf("Bench%s%d", strings.Title(fieldType), n)

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	_ = file.SetSheetRow("生成表", "A1", &[]interface{}{fmt.Sprintf("@config|Data:%s", cfgName)})

	file.NewSheet("Data")
	switch fieldType {
	case "map":
		_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性"})
		_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs"})
		_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[int32]string"})
		_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
		// 构造 N 个 map 元素："0:k0;1:k1;...;N-1:kN-1"
		for i := 0; i < n; i++ {
			parts := make([]string, n)
			for j := 0; j < n; j++ {
				parts[j] = fmt.Sprintf("%d:k%d", j, j)
			}
			_ = file.SetSheetRow("Data", fmt.Sprintf("A%d", i+5), &[]interface{}{fmt.Sprintf("%d", i), strings.Join(parts, ";")})
		}
	case "array":
		_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "网格"})
		_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Grid"})
		_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "[][]int64"})
		_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
		// 构造 N 个内层数组的二维数组："0|1|...|N-1;0|1|...|N-1"
		inner := make([]string, n)
		for j := 0; j < n; j++ {
			inner[j] = fmt.Sprintf("%d", j)
		}
		innerStr := strings.Join(inner, "|")
		outer := make([]string, n)
		for j := 0; j < n; j++ {
			outer[j] = innerStr
		}
		val := strings.Join(outer, ";")
		for i := 0; i < n; i++ {
			_ = file.SetSheetRow("Data", fmt.Sprintf("A%d", i+5), &[]interface{}{fmt.Sprintf("%d", i), val})
		}
	case "external":
		_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "结算"})
		_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "EndInfo"})
		_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "pb.TexasGameEndInfo"})
		_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
		// TexasGameEndInfo: uid,chair_id,chips,win_chips,card_type,hands,bests
		// hands/bests 用 / 分隔，放 N 个元素
		handsParts := make([]string, n)
		for j := 0; j < n; j++ {
			handsParts[j] = fmt.Sprintf("%d", j)
		}
		hands := strings.Join(handsParts, "/")
		val := fmt.Sprintf("1001,2,5000,1000,8,%s,%s", hands, hands)
		for i := 0; i < n; i++ {
			_ = file.SetSheetRow("Data", fmt.Sprintf("A%d", i+5), &[]interface{}{fmt.Sprintf("%d", i), val})
		}
	}

	if err := file.SaveAs(xlsxPath); err != nil {
		b.Fatalf("save xlsx: %v", err)
	}
	return xlsxPath, cfgName
}

// runBenchPipeline 跑一次完整的解析+生成链路（含 LoadExternalProtos，诚实计入开销）。
func runBenchPipeline(b *testing.B, xlsxPath, outDir, protoSrcDir string) {
	b.Helper()
	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.JsonPath = filepath.Join(outDir, "json")
	domain.TextPath = ""
	domain.ProtoPath = ""
	domain.BytesPath = ""
	domain.LuaPath = ""
	domain.CodePath = ""
	domain.ConfMode = "all"
	domain.PkgName = ""
	domain.PbPath = ""

	// 每轮 Clear + 重新加载外部 proto（IO 计入测量，反映真实使用场景）
	manager.Clear()
	if protoSrcDir != "" {
		if err := manager.LoadExternalProtos(protoSrcDir); err != nil {
			b.Fatalf("LoadExternalProtos: %v", err)
		}
	}

	if err := parser.ParseFiles(xlsxPath); err != nil {
		b.Fatalf("ParseFiles: %v", err)
	}
	if err := service.GenProto(); err != nil {
		b.Fatalf("GenProto: %v", err)
	}
	if err := manager.ParseProto(); err != nil {
		b.Fatalf("ParseProto: %v", err)
	}
	if err := service.GenData(); err != nil {
		b.Fatalf("GenData: %v", err)
	}
}

// BenchmarkParseMapField map 字段解析热点（N 个 map 元素 × N 行）。
func BenchmarkParseMapField(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows=%d_elems=%d", n, n), func(b *testing.B) {
			tmpDir := b.TempDir()
			xlsxPath, _ := buildBenchXlsx(b, tmpDir, n, "map")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runBenchPipeline(b, xlsxPath, tmpDir, "")
			}
		})
	}
}

// BenchmarkParseMultiArray 多维数组切分热点（N×N 二维数组 × N 行）。
func BenchmarkParseMultiArray(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows=%d_dim=%dx%d", n, n, n), func(b *testing.B) {
			tmpDir := b.TempDir()
			xlsxPath, _ := buildBenchXlsx(b, tmpDir, n, "array")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runBenchPipeline(b, xlsxPath, tmpDir, "")
			}
		})
	}
}

// BenchmarkParseExternalMessage 外部 message 反射赋值热点（含 N 个 repeated 元素 × N 行）。
func BenchmarkParseExternalMessage(b *testing.B) {
	protoSrcDir := filepath.Join(repoRootB(b), "game_protocol")
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows=%d_repeated=%d", n, n), func(b *testing.B) {
			tmpDir := b.TempDir()
			xlsxPath, _ := buildBenchXlsx(b, tmpDir, n, "external")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runBenchPipeline(b, xlsxPath, tmpDir, protoSrcDir)
			}
		})
	}
}

// BenchmarkFullPipeline 端到端全链路基线（固定小规模 map，度量整体开销）。
func BenchmarkFullPipeline(b *testing.B) {
	const n = 10
	b.Run("map+array+external_baseline", func(b *testing.B) {
		tmpDir := b.TempDir()
		xlsxMap, _ := buildBenchXlsx(b, tmpDir, n, "map")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runBenchPipeline(b, xlsxMap, tmpDir, "")
			runBenchPipeline(b, xlsxMap, tmpDir, "") // array/external 简化为重复 map（避免目录竞争）
		}
	})
}
