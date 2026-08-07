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

// TestMapFields 覆盖 map[K]V 字段的端到端生成：
//   - map[int32]string  （标量 K + 标量 V）
//   - map[string]int64  （string K + 标量 V）
//   - map[int32]Reward  （标量 K + 结构体 V，验证 K 与结构体内部 ':' 正确切分）
//
// 断言 proto 文本含 map<...> 声明，且 JSON 输出中 map 字段为对象结构。
func TestMapFields(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "map_field.xlsx")
	jsonDir := filepath.Join(tmpDir, "json")

	file := excelize.NewFile()
	defer file.Close()

	// 生成表
	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@struct|RewardSheet:Reward",
		"@config|Data:MapConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	// Reward 结构体：ItemId:Count
	file.NewSheet("RewardSheet")
	_ = file.SetSheetRow("RewardSheet", "A1", &[]interface{}{"道具ID", "数量"})
	_ = file.SetSheetRow("RewardSheet", "A2", &[]interface{}{"ItemId", "Count"})
	_ = file.SetSheetRow("RewardSheet", "A3", &[]interface{}{"int32", "int32"})
	_ = file.SetSheetRow("RewardSheet", "A4", &[]interface{}{"", ""})

	// 配置表：含三种 map 字段
	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性名映射", "标签数值", "ID奖励"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs", "Tags", "Rewards"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[int32]string", "map[string]int64", "map[int32]Reward"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all", "all", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{
		"1",
		"1:hp,2:mp,3:atk",       // int32 -> string
		"hp:100,mp:50,speed:20", // string -> int64
		"1:1:10|2:2:20",         // int32 -> Reward{ItemId:Count}，| 分隔元素，内部 ':' 需正确切分
	})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	// 配置 domain 全局变量
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

	// 断言 proto 含 map<K, V> 声明（注意：标量 V 用 , 分隔，结构体 V 直接用结构体名）
	for _, want := range []string{
		"map<int32, string> Attrs",
		"map<string, int64> Tags",
		"map<int32, Reward> Rewards",
	} {
		if !strings.Contains(protoText, want) {
			t.Fatalf("generated proto missing %q\n%s", want, protoText)
		}
	}

	// 解析 proto 描述符并生成数据
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("parse proto descriptors: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("gen data: %v", err)
	}

	jsonBytes, err := os.ReadFile(filepath.Join(jsonDir, "MapConfig.json"))
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

	// map[int32]string -> {"1":"hp","2":"mp","3":"atk"}
	attrs, ok := row["Attrs"].(map[string]interface{})
	if !ok {
		t.Fatalf("Attrs 不是 map 类型: %#v", row["Attrs"])
	}
	if attrs["1"] != "hp" || attrs["2"] != "mp" || attrs["3"] != "atk" {
		t.Fatalf("Attrs 内容不符: %#v", attrs)
	}

	// map[string]int64 -> {"hp":100,"mp":50,"speed":20}
	tags, ok := row["Tags"].(map[string]interface{})
	if !ok {
		t.Fatalf("Tags 不是 map 类型: %#v", row["Tags"])
	}
	if tags["hp"].(float64) != 100 || tags["mp"].(float64) != 50 || tags["speed"].(float64) != 20 {
		t.Fatalf("Tags 内容不符: %#v", tags)
	}

	// map[int32]Reward -> {"1":{"ItemId":1,"Count":10},"2":{"ItemId":2,"Count":20}}
	rewards, ok := row["Rewards"].(map[string]interface{})
	if !ok {
		t.Fatalf("Rewards 不是 map 类型: %#v", row["Rewards"])
	}
	if len(rewards) != 2 {
		t.Fatalf("Rewards 元素数应为 2: %#v", rewards)
	}
	r1, ok := rewards["1"].(map[string]interface{})
	if !ok {
		t.Fatalf("Rewards[1] 不是结构体 map: %#v", rewards["1"])
	}
	if r1["ItemId"].(float64) != 1 || r1["Count"].(float64) != 10 {
		t.Fatalf("Rewards[1] 内容不符: %#v", r1)
	}
	r2, _ := rewards["2"].(map[string]interface{})
	if r2["ItemId"].(float64) != 2 || r2["Count"].(float64) != 20 {
		t.Fatalf("Rewards[2] 内容不符: %#v", r2)
	}
}

// TestMapFieldsEdgeCases 覆盖空值与单元素边界。
func TestMapFieldsEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "map_edge.xlsx")
	jsonDir := filepath.Join(tmpDir, "json")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@config|Data:EdgeConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[int32]string"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
	// 两行：第一行空 map，第二行单元素 map
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{"1", ""})
	_ = file.SetSheetRow("Data", "A6", &[]interface{}{"2", "1:only"})

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
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("parse proto descriptors: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("gen data: %v", err)
	}

	jsonBytes, err := os.ReadFile(filepath.Join(jsonDir, "EdgeConfig.json"))
	if err != nil {
		t.Fatalf("read json: %v", err)
	}

	var out []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &out); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("unexpected row count: %d", len(out))
	}

	// 第一行空 map：JSON 里应为空对象（dynamic 对空 map 输出 {}）
	if _, ok := out[0]["Attrs"].(map[string]interface{}); !ok {
		t.Fatalf("空 map 应输出为空对象, 实际: %#v", out[0]["Attrs"])
	}

	// 第二行单元素 map
	attrs, ok := out[1]["Attrs"].(map[string]interface{})
	if !ok {
		t.Fatalf("单元素 map 输出类型错误: %#v", out[1]["Attrs"])
	}
	if attrs["1"] != "only" {
		t.Fatalf("单元素 map 内容不符: %#v", attrs)
	}
}

// TestMapFieldsRejectInvalidKey 验证非标量 K（如 float）在解析期被拒绝。
func TestMapFieldsRejectInvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "map_badkey.xlsx")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@config|Data:BadKeyConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[double]string"}) // float K 应被拒绝
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{"1", "1.0:a"})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	domain.XlsxPath = tmpDir
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

	// 解析阶段 buildField 应对 float K 报错（错误被 logx.Errorf 记录，字段被跳过），
	// 因此 proto 里 Attrs 字段不应出现 map<double, string>
	fileName := strings.TrimSuffix(filepath.Base(xlsxPath), filepath.Ext(xlsxPath))
	protoText := manager.GetProtoMap()[base.GetProtoName(fileName)]
	if protoText == "" {
		t.Fatalf("missing generated proto for %s", fileName)
	}
	if strings.Contains(protoText, "map<double, string>") {
		t.Fatalf("float K 应被拒绝，但 proto 中出现了 map<double, string>:\n%s", protoText)
	}
}
