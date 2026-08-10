package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
)

// 本文件覆盖「@命名 + 无生成表自动注册」改动：
//   1. 掉落系统@drop.xlsx（无生成表）-> 自动注册所有数据 sheet 为 config
//   2. 类型名 = ConfigTypeName(feature, table)（drop+group -> DropGroupConfig）
//   3. proto 分桶 = 功能名（drop.proto）
//   4. Go 代码目录 = feature（drop/），文件名 = gdata_<feature>_<table>.go
//   5. 同名去重（feature=table -> ItemConfig）

// TestAutoRegister_AtNaming 核心：@文件名 + @sheet 名，无生成表自动注册。
func TestAutoRegister_AtNaming(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落组表@group": {
			{"掉落组Id", "子序号", "掉落组名"},
			{"Groupid", "Subid", "Name"},
			{"int32", "int32", "string"},
			{"key", "all", "all"},
			{"1001", "1", "叶子掉落包A"},
		},
		"掉落表@item": {
			{"掉落Id", "道具Id", "数量"},
			{"DropId", "ItemId", "Count"},
			{"int32", "int32", "int32"},
			{"key", "all", "all"},
			{"2001", "1001", "5"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.ConfMode = "all"
	domain.ProtoSrcPath = ""
	resetGlobals(t)

	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}

	// 断言 config 注册：类型名 = feature+table+Config
	cfgNames := map[string]bool{}
	for _, c := range manager.GetConfigList() {
		cfgNames[c.Name] = true
		t.Logf("config: Name=%s FileName=%s Feature=%s Table=%s", c.Name, c.FileName, c.Feature, c.Table)
		if c.FileName != "drop" {
			t.Errorf("FileName 应为 drop, got %q", c.FileName)
		}
		if c.Feature != "drop" {
			t.Errorf("Feature 应为 drop, got %q", c.Feature)
		}
	}
	if !cfgNames["DropGroupConfig"] {
		t.Error("缺少 DropGroupConfig（应自动注册）")
	}
	if !cfgNames["DropItemConfig"] {
		t.Error("缺少 DropItemConfig（应自动注册）")
	}
}

// TestAutoRegister_ProtoBucket 自动注册的 proto 分桶 = 功能名。
func TestAutoRegister_ProtoBucket(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "德州@texas.xlsx")

	dataSheets := map[string][][]interface{}{
		"德州扑克表@texas": {
			{"房间类型", "货币类型"},
			{"RoomStage", "CoinType"},
			{"int32", "int32"},
			{"key", "key"},
			{"1", "1"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	protoText, _ := runFullPipeline(t, xlsxPath, tmpDir)
	if !strings.Contains(protoText, "message TexasConfig") {
		t.Errorf("proto 应含 message TexasConfig（同名去重 feature=table）:\n%s", protoText)
	}
}

// TestAutoRegister_GoCodeLayout 生成的 Go 代码目录/文件名符合 gdata_ 命名。
func TestAutoRegister_GoCodeLayout(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落组表@group": {
			{"掉落组Id", "子序号"},
			{"Groupid", "Subid"},
			{"int32", "int32"},
			{"key", "all"},
			{"1001", "1"},
		},
		"掉落表@item": {
			{"掉落Id", "道具Id"},
			{"DropId", "ItemId"},
			{"int32", "int32"},
			{"key", "all"},
			{"2001", "1001"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 2 {
		t.Fatalf("应生成 2 个 Go 文件, got %d: %v", len(codeFiles), codeFiles)
	}

	wantPaths := map[string]bool{
		filepath.Join(tmpDir, "code", "drop", "gdata_drop_group.go"): false,
		filepath.Join(tmpDir, "code", "drop", "gdata_drop_item.go"):  false,
	}
	for _, f := range codeFiles {
		if _, ok := wantPaths[f]; ok {
			wantPaths[f] = true
		}
	}
	for p, ok := range wantPaths {
		if !ok {
			t.Errorf("缺少生成文件: %s", p)
		}
	}
}

// TestAutoRegister_DeDup 同名去重：道具@item.xlsx + 道具表@item -> ItemConfig。
func TestAutoRegister_DeDup(t *testing.T) {
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
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	protoText, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if !strings.Contains(protoText, "message ItemConfig") {
		t.Errorf("同名去重应生成 message ItemConfig（而非 ItemItemConfig）:\n%s", protoText)
	}
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	// 文件名：gdata_<feature>_<table>.go（不因去重而省略）
	if !strings.HasSuffix(codeFiles[0], filepath.Join("item", "gdata_item_item.go")) {
		t.Errorf("文件名应为 gdata_item_item.go, got %s", codeFiles[0])
	}
}

// TestAutoRegister_SkipHelperSheet 跳过 _ 开头的辅助 sheet。
func TestAutoRegister_SkipHelperSheet(t *testing.T) {
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
		"_说明": {
			{"这是一个辅助 sheet，不应被注册"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.ConfMode = "all"
	resetGlobals(t)
	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if len(manager.GetConfigList()) != 1 {
		t.Errorf("应只注册 1 个 config（_说明 应被跳过）, got %d", len(manager.GetConfigList()))
	}
}

// TestAutoRegister_GoKeywordFeature const 是 Go 关键字，功能名退避验证（consts 可正常生成 package）。
func TestAutoRegister_GoKeywordFeature(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "常量@consts.xlsx")

	dataSheets := map[string][][]interface{}{
		"全局常量表@global": {
			{"字段名", "值"},
			{"Name", "Value"},
			{"string", "int32"},
			{"key", "all"},
			{"MAX_LEVEL", "100"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	content := readGoFile(t, codeFiles[0])
	if !strings.Contains(content, "package consts") {
		t.Errorf("package 应为 consts（Go 关键字规避）:\n%s", content)
	}
	if !strings.Contains(content, "ConstsGlobalConfig") {
		t.Errorf("应含 ConstsGlobalConfig 类型:\n%s", content)
	}
	_ = base.ConfigTypeName
}
