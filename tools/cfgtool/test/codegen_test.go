package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
)

// 本文件覆盖「interface 包装 + 便捷函数 + 复合键」改动（code_tpl.go / func.go / index.go）：
//   1. 生成 I{Prefix} interface 定义
//   2. 包级单例 {Prefix} I{Prefix}
//   3. 包级函数代理（Get{Prefix}ByXxx 兼容旧调用）
//   4. 便捷函数 MustGet / Has / GetMap
//   5. group 索引 GroupByXxxFunc（索引内二次筛选）
//   6. 复合键用 gamedata.Index2（非 protocol.Index2）+ 命名字段初始化
//   7. 加载时重复主键检测

// TestCodegen_InterfaceAndProxy 生成代码含 interface、单例、包级代理。
func TestCodegen_InterfaceAndProxy(t *testing.T) {
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
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	// interface 定义
	if !strings.Contains(code, "type IDropGroup interface {") {
		t.Error("缺少 IDropGroup interface 定义")
	}
	// 包级单例
	if !strings.Contains(code, "var DropGroup IDropGroup = &DropGroupImpl{}") {
		t.Error("缺少包级单例 DropGroup")
	}
	// 包级函数代理（兼容旧调用）
	for _, proxy := range []string{
		"func GetDropGroupByGroupid",
		"func MustGetDropGroupByGroupid",
		"func HasDropGroupByGroupid",
	} {
		if !strings.Contains(code, proxy) {
			t.Errorf("缺少包级代理 %s", proxy)
		}
	}
	// init 注册保持热更机制
	if !strings.Contains(code, `gamedata.Register("DropGroupConfig"`) {
		t.Error("缺少 gamedata.Register 注册")
	}
}

// TestCodegen_ConvenienceMethods map 主键索引的便捷函数（MustGet/Has/GetMap）。
func TestCodegen_ConvenienceMethods(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落表@item": {
			{"掉落道具Id", "道具Id", "数量"},
			{"DropItemId", "ItemId", "Count"},
			{"int32", "int32", "int32"},
			{"key", "all", "all"},
			{"2001", "1001", "5"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	// 注意：类型前缀是 g1_protocol（domain.PkgName），故只匹配方法签名去前缀部分
	for _, want := range []string{
		"func (c *DropItemImpl) GetByDropItemId(DropItemId int32)",
		"func (c *DropItemImpl) MustGetByDropItemId(DropItemId int32)",
		"func (c *DropItemImpl) HasByDropItemId(DropItemId int32) bool",
		"func (c *DropItemImpl) GetMapDropItemId() map[int32]",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("缺少便捷函数:\n  %s\n--- 代码 ---\n%s", want, code)
		}
	}
	// MustGet panic 提示含配置名
	if !strings.Contains(code, `panic(fmt.Sprintf("DropItemConfig 主键 DropItemId`) {
		t.Error("MustGet panic 提示应含配置名与主键名")
	}
}

// TestCodegen_GroupByFunc group 索引生成 GroupByXxxFunc（索引内二次筛选）。
func TestCodegen_GroupByFunc(t *testing.T) {
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
	}
	makeXLSX(t, xlsxPath, []string{"@config|掉落组表@group:DropGroupConfig|group:Groupid"}, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	for _, want := range []string{
		"func (c *DropGroupImpl) GroupByGroupid(Groupid int32)",
		"func (c *DropGroupImpl) GroupByGroupidFunc(Groupid int32, fn func(",
		"func GroupDropGroupByGroupid(Groupid int32)",
		"func GroupDropGroupByGroupidFunc(Groupid int32, fn func(",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("缺少 group 索引函数:\n  %s\n--- 代码 ---\n%s", want, code)
		}
	}
}

// TestCodegen_CompositeKey 复合键（多字段 key）用 gamedata.Index2 + 命名字段初始化。
func TestCodegen_CompositeKey(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "德州@texas.xlsx")

	dataSheets := map[string][][]interface{}{
		"德州扑克表@texas": {
			{"房间类型", "货币类型", "小盲注"},
			{"RoomStage", "CoinType", "SmallBlind"},
			{"int32", "int32", "int32"},
			{"key", "key", "all"},
			{"1", "1", "10"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	// 复合键类型：gamedata.Index2（非 protocol.Index2）
	if !strings.Contains(code, "gamedata.Index2[int32, int32]") {
		t.Errorf("复合键应用 gamedata.Index2:\n%s", code)
	}
	if strings.Contains(code, "protocol.Index2") {
		t.Error("不应再引用 protocol.Index2（已内置 gamedata）")
	}
	// 命名字段初始化（消除 vet unkeyed warning）
	if !strings.Contains(code, "gamedata.Index2[int32, int32]{T2: item.RoomStage, T1: item.CoinType}") {
		t.Errorf("复合键字面量应为命名字段初始化:\n%s", code)
	}
	// 复合键查询方法
	if !strings.Contains(code, "func (c *TexasImpl) GetByRoomStageCoinType(RoomStage int32, CoinType int32)") {
		t.Errorf("缺少复合键查询方法:\n%s", code)
	}
}

// TestCodegen_DuplicateKeyDetect 加载时重复主键检测（parse 返回 error）。
func TestCodegen_DuplicateKeyDetect(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落表@item": {
			{"掉落Id", "道具Id"},
			{"DropId", "ItemId"},
			{"int32", "int32"},
			{"key", "all"},
			{"2001", "1001"},
			{"2001", "1002"}, // 重复主键
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])
	if !strings.Contains(code, "重复主键") {
		t.Errorf("生成代码应含重复主键检测:\n%s", code)
	}
	if !strings.Contains(code, "; exists {") {
		t.Errorf("重复主键检测应判断 exists:\n%s", code)
	}
}

// TestCodegen_LegacyCompat 无 @ 旧式文件名 + 生成表规则：退化为旧行为（package 由 Name 推导）。
func TestCodegen_LegacyCompat(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "legacy.xlsx")

	dataSheets := map[string][][]interface{}{
		"Data": {
			{"编号", "名称"},
			{"Id", "Name"},
			{"int32", "string"},
			{"key", "all"},
			{"1", "test"},
		},
	}
	// 旧式生成表：@config|sheet:结构名（无 @ 命名）
	makeXLSX(t, xlsxPath, []string{"@config|Data:LegacyConfig"}, dataSheets)

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.ConfMode = "all"
	resetGlobals(t)
	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	found := false
	for _, c := range manager.GetConfigList() {
		if c.Name == "LegacyConfig" {
			found = true
			if c.Feature != "" {
				t.Errorf("无 @ 文件 Feature 应为空, got %q", c.Feature)
			}
		}
	}
	if !found {
		t.Error("旧式生成表应注册 LegacyConfig")
	}
}
