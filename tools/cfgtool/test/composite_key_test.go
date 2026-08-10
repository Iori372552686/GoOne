package test

import (
	"path/filepath"
	"strings"
	"testing"
)

// 本文件覆盖 3/4 维复合键（Index3/Index4）生成正确性：
//   1. 3 字段 key -> gamedata.Index3 + 命名字段初始化（T3:/T2:/T1:）
//   2. 4 字段 key -> gamedata.Index4 + 命名字段初始化（T4:/T3:/T2:/T1:）
//   3. 复合查询方法 / MustGet panic 参数数 / 重复主键检测
//   4. 生成代码语法合法（go/parser 解析）

// TestCompositeKey_Index3 三字段复合键。
func TestCompositeKey_Index3(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落组表@group": {
			{"掉落组Id", "子序号", "组类型", "掉落组名"},
			{"Groupid", "Subid", "Type", "Name"},
			{"int32", "int32", "int32", "string"},
			{"key", "key", "key", "all"},
			{"1001", "1", "0", "叶子A"},
			{"1001", "2", "1", "组合B"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	// Index3 引用
	if !strings.Contains(code, "gamedata.Index3[int32, int32, int32]") {
		t.Errorf("3 字段 key 应用 gamedata.Index3:\n%s", code)
	}
	// 命名字段初始化（顺序：List[0]->T3, List[1]->T2, List[2]->T1）
	if !strings.Contains(code, "gamedata.Index3[int32, int32, int32]{T3: item.Groupid, T2: item.Subid, T1: item.Type}") {
		t.Errorf("Index3 命名字段初始化错误:\n%s", code)
	}
	// 复合查询方法（3 参数）
	if !strings.Contains(code, "func (c *DropGroupImpl) GetByGroupidSubidType(Groupid int32, Subid int32, Type int32)") {
		t.Errorf("缺少 Index3 复合查询方法:\n%s", code)
	}
	// MustGet panic 提示：3 个 %v 匹配 3 个参数
	if !strings.Contains(code, `"DropGroupConfig 主键 GroupidSubidType=%v, %v, %v 不存在", Groupid, Subid, Type)`) {
		t.Errorf("Index3 MustGet panic 提示参数数应匹配:\n%s", code)
	}
	// 重复复合主键检测
	if !strings.Contains(code, "DropGroupConfig 重复复合主键 GroupidSubidType") {
		t.Errorf("Index3 应含重复复合主键检测:\n%s", code)
	}
	// 语法合法
	if err := gofmtValid(code); err != nil {
		t.Errorf("Index3 生成代码语法不合法: %v", err)
	}
}

// TestCompositeKey_Index4 四字段复合键。
func TestCompositeKey_Index4(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "德州@texas.xlsx")

	dataSheets := map[string][][]interface{}{
		"德州扑克表@texas": {
			{"房间类型", "货币类型", "座位号", "轮次", "小盲注"},
			{"RoomStage", "CoinType", "Seat", "Round", "SmallBlind"},
			{"int32", "int32", "int32", "int32", "int32"},
			{"key", "key", "key", "key", "all"},
			{"1", "1", "1", "1", "10"},
			{"1", "1", "1", "2", "10"},
		},
	}
	makeXLSX(t, xlsxPath, nil, dataSheets)

	_, codeFiles := runFullPipeline(t, xlsxPath, tmpDir)
	if len(codeFiles) != 1 {
		t.Fatalf("应生成 1 个 Go 文件, got %d", len(codeFiles))
	}
	code := readGoFile(t, codeFiles[0])

	// Index4 引用
	if !strings.Contains(code, "gamedata.Index4[int32, int32, int32, int32]") {
		t.Errorf("4 字段 key 应用 gamedata.Index4:\n%s", code)
	}
	// 命名字段初始化（顺序：List[0]->T4 ... List[3]->T1）
	if !strings.Contains(code, "gamedata.Index4[int32, int32, int32, int32]{T4: item.RoomStage, T3: item.CoinType, T2: item.Seat, T1: item.Round}") {
		t.Errorf("Index4 命名字段初始化错误:\n%s", code)
	}
	// 复合查询方法（4 参数）
	if !strings.Contains(code, "func (c *TexasImpl) GetByRoomStageCoinTypeSeatRound(RoomStage int32, CoinType int32, Seat int32, Round int32)") {
		t.Errorf("缺少 Index4 复合查询方法:\n%s", code)
	}
	// MustGet panic 提示：4 个 %v 匹配 4 个参数
	if !strings.Contains(code, `"TexasConfig 主键 RoomStageCoinTypeSeatRound=%v, %v, %v, %v 不存在", RoomStage, CoinType, Seat, Round)`) {
		t.Errorf("Index4 MustGet panic 提示参数数应匹配:\n%s", code)
	}
	// 重复复合主键检测
	if !strings.Contains(code, "TexasConfig 重复复合主键 RoomStageCoinTypeSeatRound") {
		t.Errorf("Index4 应含重复复合主键检测:\n%s", code)
	}
	// 语法合法
	if err := gofmtValid(code); err != nil {
		t.Errorf("Index4 生成代码语法不合法: %v", err)
	}
}

// TestCompositeKey_MixedDims 同一文件 2 维+3 维混合（确保互不干扰）。
func TestCompositeKey_MixedDims(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "掉落系统@drop.xlsx")

	dataSheets := map[string][][]interface{}{
		"掉落组表@group": {
			{"掉落组Id", "子序号", "组类型"},
			{"Groupid", "Subid", "Type"},
			{"int32", "int32", "int32"},
			{"key", "key", "key"},
			{"1001", "1", "0"},
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
		t.Fatalf("应生成 2 个 Go 文件, got %d", len(codeFiles))
	}
	for _, f := range codeFiles {
		code := readGoFile(t, f)
		switch {
		case strings.Contains(code, "DropGroupConfig"):
			if !strings.Contains(code, "gamedata.Index3[int32, int32, int32]") {
				t.Errorf("DropGroupConfig 应用 Index3:\n%s", code)
			}
		case strings.Contains(code, "DropItemConfig"):
			if !strings.Contains(code, "map[int32]") || strings.Contains(code, "Index") {
				t.Errorf("DropItemConfig 单字段 key 不应含 Index:\n%s", code)
			}
		}
		if err := gofmtValid(code); err != nil {
			t.Errorf("语法不合法 %s: %v", f, err)
		}
	}
}
