package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
)

// 本文件覆盖「生成表规则与自动注册共存」改动（parser.go 混合模式）：
//   1. 有生成表时，声明了额外规则（group 索引）的 sheet 走规则
//   2. 未声明的数据 sheet 仍自动注册为 config
//   3. 生成表里显式指定的结构名优先于自动命名
//   4. @enum 声明枚举 sheet（FileName 跟随功能名）

// TestHybrid_GenTablePlusAutoRegister 生成表声明 group 规则 + 其余 sheet 自动注册。
func TestHybrid_GenTablePlusAutoRegister(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "道具@item.xlsx")

	dataSheets := map[string][][]interface{}{
		"道具表@item": {
			{"道具Id", "道具名"},
			{"ItemId", "Name"},
			{"int32", "string"},
			{"key", "all"},
			{"1001", "金币"},
		},
		"分解表@decompose": {
			{"Id", "被分解道具Id", "产出道具Id"},
			{"Id", "ItemId", "OutputItemId"},
			{"int32", "int32", "int32"},
			{"key", "all", "all"},
			{"1", "1001", "2001"},
		},
	}
	// 生成表只为分解表声明 group:ItemId（一对多），道具表未声明应自动注册
	makeXLSX(t, xlsxPath, []string{"@config|分解表@decompose:ItemDecomposeConfig|group:ItemId"}, dataSheets)

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.ConfMode = "all"
	resetGlobals(t)
	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}

	cfgNames := map[string]bool{}
	for _, c := range manager.GetConfigList() {
		cfgNames[c.Name] = true
	}
	if !cfgNames["ItemConfig"] {
		t.Error("未声明的 道具表@item 应自动注册为 ItemConfig")
	}
	if !cfgNames["ItemDecomposeConfig"] {
		t.Error("生成表声明的 分解表 应注册为 ItemDecomposeConfig")
	}
	if len(cfgNames) != 2 {
		t.Errorf("应恰好注册 2 个 config, got %d: %v", len(cfgNames), cfgNames)
	}
}

// TestHybrid_GroupIndex 生成表 group 规则生效：生成 GroupByXxx + GroupByXxxFunc。
func TestHybrid_GroupIndex(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "道具@item.xlsx")

	dataSheets := map[string][][]interface{}{
		"分解表@decompose": {
			{"Id", "被分解道具Id", "产出道具Id"},
			{"Id", "ItemId", "OutputItemId"},
			{"int32", "int32", "int32"},
			{"key", "all", "all"},
			{"1", "1001", "2001"},
			{"2", "1001", "2002"}, // 同 ItemId 多条 -> group 语义
		},
	}
	makeXLSX(t, xlsxPath, []string{"@config|分解表@decompose:ItemDecomposeConfig|group:ItemId"}, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])
	for _, want := range []string{
		"func (c *ItemDecomposeImpl) GroupByItemId(ItemId int32)",
		"func (c *ItemDecomposeImpl) GroupByItemIdFunc(ItemId int32, fn func(",
		"func GroupItemDecomposeByItemId(ItemId int32)",
		"func GroupItemDecomposeByItemIdFunc(ItemId int32, fn func(",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("缺少 group 索引函数:\n  %s", want)
		}
	}
}

// TestHybrid_EnumSheet @enum 声明枚举：FileName 跟随功能名（enum.proto）。
func TestHybrid_EnumSheet(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "枚举@enum.xlsx")

	dataSheets := map[string][][]interface{}{
		"枚举表@enum": {
			// proto3 要求第一个枚举值为 0
			{"E|道具类型-空|PropertyType|Empty|0"},
			{"E|道具类型-金币|PropertyType|Coin|1"},
		},
	}
	// 生成表声明枚举 sheet
	makeXLSX(t, xlsxPath, []string{"@enum|枚举表@enum"}, dataSheets)

	protoText, _ := runFullPipeline(t, xlsxPath, tmpDir)
	if !strings.Contains(protoText, "enum PropertyType") {
		t.Errorf("proto 应含枚举 PropertyType:\n%s", protoText)
	}
	if !strings.Contains(protoText, "PropertyType_Coin = 1") {
		t.Errorf("枚举值应为 PropertyType_Coin = 1:\n%s", protoText)
	}
	if !strings.Contains(protoText, "PropertyType_Empty = 0") {
		t.Errorf("枚举首值应为 PropertyType_Empty = 0（proto3 规范）:\n%s", protoText)
	}
}
