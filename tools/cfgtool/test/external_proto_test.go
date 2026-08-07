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

// TestExternalProtoReference 端到端验证 pb.XXX 外部 proto 引用：
//   - 在 tempDir 构造外部 proto 目录（含 message TheReward + enum TheRarity）
//   - 构造 xlsx 配置表引用 pb.TheReward（单值/数组/map value）
//   - 断言 proto 含 import 外部 proto；JSON 数据正确填充
func TestExternalProtoReference(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 准备外部 proto 源目录
	//    目录结构: <tmpDir>/proto/ext/reward.proto
	//    import 路径将相对为 "proto/ext/reward.proto"
	protoSrcDir := filepath.Join(tmpDir, "proto", "ext")
	if err := os.MkdirAll(protoSrcDir, 0o755); err != nil {
		t.Fatalf("mkdir proto src: %v", err)
	}
	externalProto := `syntax = "proto3";
package g1.protocol;
option go_package = "./g1_protocol";

// 测试用外部结构体
message TheReward {
  int32 item_id = 1;
  int64 count   = 2;
  TheRarity rarity = 3;
}

// 测试用外部枚举
enum TheRarity {
  TheNormal = 0;
  TheRare   = 1;
  TheEpic   = 2;
}
`
	protoFile := filepath.Join(protoSrcDir, "reward.proto")
	if err := os.WriteFile(protoFile, []byte(externalProto), 0o644); err != nil {
		t.Fatalf("write external proto: %v", err)
	}

	// 2. 准备 xlsx
	xlsxPath := filepath.Join(tmpDir, "ext_ref.xlsx")
	jsonDir := filepath.Join(tmpDir, "json")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@config|Data:ExtConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "单奖励", "奖励列表", "ID奖励映射"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Reward", "Rewards", "RewardMap"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "pb.TheReward", "[]pb.TheReward", "map[int32]pb.TheReward"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all", "all", "all"})
	// 数据：TheReward 字段顺序 item_id,count,rarity
	//   单奖励:      100,200,1
	//   奖励列表:    100,200,1|200,300,2  (数组元素 '|' 分隔，成员 ',')
	//   ID奖励映射:  1:100,200,1;2:200,300,2 (map 元素 ';', K:V ':', 成员 ',')
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{
		"1",
		"100,200,1",
		"100,200,1|200,300,2",
		"1:100,200,1;2:200,300,2",
	})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	// 3. 配置 domain 全局变量
	domain.XlsxPath = tmpDir
	domain.JsonPath = jsonDir
	domain.ProtoSrcPath = tmpDir // 外部 proto 根（相对路径将从这里计算）
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

	// 4. 执行主流程（手动编排，对应 main.go run()）
	if err := manager.LoadExternalProtos(domain.ProtoSrcPath); err != nil {
		t.Fatalf("load external protos: %v", err)
	}
	// 验证外部类型已注册
	if !manager.IsExternalMsg("TheReward") {
		t.Fatalf("TheReward 未注册到 externalMsgMgr")
	}
	if !manager.IsExternalEnum("TheRarity") {
		t.Fatalf("TheRarity 未注册到 externalEnumMgr")
	}

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

	// 断言 proto 含 import 外部 proto（剥 .proto 后的基名）
	// reward.proto 相对 ProtoSrcPath 的路径是 proto/ext/reward
	if !strings.Contains(protoText, `import "proto/ext/reward.proto";`) {
		t.Fatalf("proto 缺少外部 import, got:\n%s", protoText)
	}
	// 断言字段类型用裸短名（同包 g1.protocol）
	if !strings.Contains(protoText, "TheReward Reward") {
		t.Fatalf("proto 字段类型未用裸短名, got:\n%s", protoText)
	}
	if !strings.Contains(protoText, "repeated TheReward Rewards") {
		t.Fatalf("proto 数组字段类型异常, got:\n%s", protoText)
	}
	if !strings.Contains(protoText, "map<int32, TheReward> RewardMap") {
		t.Fatalf("proto map 字段类型异常, got:\n%s", protoText)
	}

	// 5. 解析 proto 描述符 + 生成数据
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("parse proto descriptors: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("gen data: %v", err)
	}

	// 6. 验证 JSON 输出
	jsonBytes, err := os.ReadFile(filepath.Join(jsonDir, "ExtConfig.json"))
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

	// 单奖励 TheReward{item_id:100, count:200, rarity:1}
	reward, ok := row["Reward"].(map[string]interface{})
	if !ok {
		t.Fatalf("Reward 不是对象: %#v", row["Reward"])
	}
	if reward["item_id"].(float64) != 100 || reward["count"].(float64) != 200 {
		t.Fatalf("Reward 内容不符: %#v", reward)
	}

	// 奖励列表 [TheReward{100,200,1}, TheReward{200,300,2}]
	rewards, ok := row["Rewards"].([]interface{})
	if !ok {
		t.Fatalf("Rewards 不是数组: %#v", row["Rewards"])
	}
	if len(rewards) != 2 {
		t.Fatalf("Rewards 元素数应为 2: %#v", rewards)
	}
	r0 := rewards[0].(map[string]interface{})
	if r0["item_id"].(float64) != 100 || r0["count"].(float64) != 200 {
		t.Fatalf("Rewards[0] 内容不符: %#v", r0)
	}

	// map[int32]TheReward {1:{100,200,1}, 2:{200,300,2}}
	rewardMap, ok := row["RewardMap"].(map[string]interface{})
	if !ok {
		t.Fatalf("RewardMap 不是对象: %#v", row["RewardMap"])
	}
	if len(rewardMap) != 2 {
		t.Fatalf("RewardMap 元素数应为 2: %#v", rewardMap)
	}
	m1 := rewardMap["1"].(map[string]interface{})
	if m1["item_id"].(float64) != 100 || m1["count"].(float64) != 200 {
		t.Fatalf("RewardMap[1] 内容不符: %#v", m1)
	}
	m2 := rewardMap["2"].(map[string]interface{})
	if m2["item_id"].(float64) != 200 || m2["count"].(float64) != 300 {
		t.Fatalf("RewardMap[2] 内容不符: %#v", m2)
	}
}

// TestExternalProtoUnknownType 验证未注册的 pb.XXX 在解析期被拒绝。
func TestExternalProtoUnknownType(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "ext_unknown.xlsx")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@config|Data:UnknownConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "未注册类型"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Field"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "pb.NotExist"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{"1", "1"})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	domain.XlsxPath = tmpDir
	domain.ProtoSrcPath = "" // 不加载任何外部 proto
	domain.ConfMode = "all"
	domain.PkgName = ""
	domain.PbPath = ""

	resetGlobals(t)

	// pb.NotExist 未注册，ParseFiles 内 buildField 会报错并被 logx 记录，字段被跳过
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
	// 未注册类型应被跳过，proto 里不应出现 NotExist
	if strings.Contains(protoText, "NotExist") {
		t.Fatalf("未注册的 pb.NotExist 应被拒绝，但出现在 proto 中:\n%s", protoText)
	}
}
