package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
	"github.com/xuri/excelize/v2"
)

func TestGenDataSupportsMultiDimensionalArrays(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "multi_array.xlsx")
	jsonDir := filepath.Join(tmpDir, "json")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@struct|RewardSheet:Reward",
		"@config|Data:ArrConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("RewardSheet")
	_ = file.SetSheetRow("RewardSheet", "A1", &[]interface{}{"道具ID", "数量"})
	_ = file.SetSheetRow("RewardSheet", "A2", &[]interface{}{"ItemId", "Count"})
	_ = file.SetSheetRow("RewardSheet", "A3", &[]interface{}{"int32", "int32"})
	_ = file.SetSheetRow("RewardSheet", "A4", &[]interface{}{"", ""})

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "二维整数", "三维整数", "二维奖励"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Grid", "Cube", "Rewards"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "[][]int64", "[][][]int64", "[][]Reward"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all", "all", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{
		"1",
		"1|2;3|4",             // [][]int64:  1维'|', 2维';'
		"1|2;3|4^5|6;7|8",     // [][][]int64: 1维'|', 2维';', 3维'^'
		"1,10|2,20;3,30|4,40", // [][]Reward: 成员',', 1维'|', 2维';'
	})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	domain.XlsxPath = tmpDir
	domain.JsonPath = jsonDir
	domain.TextPath = ""
	domain.ProtoPath = ""
	domain.BytesPath = ""
	domain.LuaPath = ""
	domain.TsPath = ""
	domain.CodePath = ""
	domain.CppPath = ""
	domain.NodeJsPath = ""
	domain.ConfMode = "all"
	domain.PkgName = ""
	domain.PbPath = ""

	resetGlobals(t)

	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if err := service.GenProto(); err != nil {
		t.Fatalf("gen proto: %v", err)
	}

	fileName := strings.TrimSuffix(filepath.Base(xlsxPath), filepath.Ext(xlsxPath))
	protoText := manager.GetProtoMap()[base.GetProtoName(fileName)]
	if protoText == "" {
		t.Fatalf("missing generated proto for %s", fileName)
	}

	int64Array1 := base.ArrayWrapperName(fileName, "int64", 1)
	int64Array2 := base.ArrayWrapperName(fileName, "int64", 2)
	rewardArray1 := base.ArrayWrapperName(fileName, "Reward", 1)
	for _, want := range []string{
		"message " + int64Array1,
		"message " + int64Array2,
		"message " + rewardArray1,
		"repeated " + int64Array1 + " Grid = 2;",
		"repeated " + int64Array2 + " Cube = 3;",
		"repeated " + rewardArray1 + " Rewards = 4;",
	} {
		if !strings.Contains(protoText, want) {
			t.Fatalf("generated proto missing %q\n%s", want, protoText)
		}
	}

	if err := manager.ParseProto(); err != nil {
		t.Fatalf("parse proto descriptors: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("gen data: %v", err)
	}

	jsonBytes, err := os.ReadFile(filepath.Join(jsonDir, "ArrConfig.json"))
	if err != nil {
		t.Fatalf("read json: %v", err)
	}

	var out []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("unexpected row count: %d", len(out))
	}

	row := out[0]
	if row["Id"].(float64) != 1 {
		t.Fatalf("unexpected Id: %#v", row["Id"])
	}

	grid := row["Grid"].([]interface{})
	if len(grid) != 2 || len(grid[0].([]interface{})) != 2 || len(grid[1].([]interface{})) != 2 {
		t.Fatalf("unexpected Grid: %#v", row["Grid"])
	}

	cube := row["Cube"].([]interface{})
	if len(cube) != 2 {
		t.Fatalf("unexpected Cube outer len: %#v", row["Cube"])
	}
	if len(cube[0].([]interface{})) != 2 || len(cube[1].([]interface{})) != 2 {
		t.Fatalf("unexpected Cube inner len: %#v", row["Cube"])
	}

	rewards := row["Rewards"].([]interface{})
	if len(rewards) != 2 {
		t.Fatalf("unexpected Rewards outer len: %#v", row["Rewards"])
	}
	firstRewardRow := rewards[0].([]interface{})
	if len(firstRewardRow) != 2 {
		t.Fatalf("unexpected Rewards row len: %#v", row["Rewards"])
	}
	firstReward := firstRewardRow[0].(map[string]interface{})
	if firstReward["ItemId"].(float64) != 1 || firstReward["Count"].(float64) != 10 {
		t.Fatalf("unexpected first reward: %#v", firstReward)
	}
}
