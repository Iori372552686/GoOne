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

// 本文件对 cfgtool 三项新功能做综合业务测试：
//   1. map[K]V 字段支持
//   2. 分隔符统一（结构体 , / 数组 |/;/^ / map K:V :）
//   3. 外部 proto message 引用（pb.XXX + -proto-src）
//
// 外部 proto 引用真实业务结构：
//   - pb.TexasGameEndInfo（struct.proto:142，含 标量+enum+repeated）
//     字段顺序: uid(uint64) / chair_id(uint32) / chips(int64) / win_chips(int64)
//               / card_type(CardType enum) / hands(repeated uint32) / bests(repeated uint32)
//   - pb.CardType（game_enum.proto:33，enum）
//
// 测试把三项特性聚合到同一张配置表，端到端验证 proto/JSON/pb-text 三种输出格式。

// runIntegrationPipeline 执行一次完整的 cfgtool 主流程，返回 proto文本/json数据/pb-text数据。
// protoSrcDir 非空时会先 LoadExternalProtos。
func runIntegrationPipeline(t *testing.T, xlsxPath, outDir, protoSrcDir string) (protoText, jsonText, pbText string) {
	t.Helper()
	fileName := strings.TrimSuffix(filepath.Base(xlsxPath), filepath.Ext(xlsxPath))
	jsonDir := filepath.Join(outDir, "json")
	textDir := filepath.Join(outDir, "text")

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.JsonPath = jsonDir
	domain.TextPath = textDir
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
	domain.ProtoSrcPath = protoSrcDir

	resetGlobals(t)

	if protoSrcDir != "" {
		if err := manager.LoadExternalProtos(protoSrcDir); err != nil {
			t.Fatalf("LoadExternalProtos: %v", err)
		}
	}
	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if err := service.GenProto(); err != nil {
		t.Fatalf("GenProto: %v", err)
	}
	protoText = manager.GetProtoMap()[base.GetProtoName(fileName)]

	if err := manager.ParseProto(); err != nil {
		t.Fatalf("ParseProto: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("GenData: %v", err)
	}

	// 读取生成的 config name（@config 规则里的结构名）
	// 这里约定生成表第一条规则是 @config|<sheet>:<ConfigName>
	// 为简单起见，用 protoMgr 找到该文件生成的 proto 文本里的第一个 message 名
	// 实际由调用方通过 cfgName 传入更稳妥，此处从目录读取文件
	if b, err := os.ReadFile(filepath.Join(jsonDir, firstConfigName(t, xlsxPath)+".json")); err == nil {
		jsonText = string(b)
	}
	if b, err := os.ReadFile(filepath.Join(textDir, firstConfigName(t, xlsxPath)+".conf")); err == nil {
		pbText = string(b)
	}
	return
}

// firstConfigName 从 xlsx 的「生成表」解析出 @config 的结构名。
func firstConfigName(t *testing.T, xlsxPath string) string {
	t.Helper()
	fp, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		t.Fatalf("open xlsx for config name: %v", err)
	}
	defer fp.Close()
	rows, err := fp.GetRows("生成表")
	if err != nil {
		t.Fatalf("get 生成表: %v", err)
	}
	for _, row := range rows {
		for _, cell := range row {
			if strings.HasPrefix(cell, "@config") {
				parts := strings.Split(cell, "|")
				for _, p := range parts {
					if idx := strings.Index(p, ":"); idx > 0 {
						return p[idx+1:]
					}
				}
			}
		}
	}
	t.Fatal("未在生成表找到 @config 规则")
	return ""
}

// TestIntegration_AllFeatures 三特性聚合用例：单一 config 覆盖 map/分隔符/外部proto。
func TestIntegration_AllFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "integration.xlsx")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	if err := file.SetSheetRow("生成表", "A1", &[]interface{}{
		"@config|Data:IntConfig",
	}); err != nil {
		t.Fatalf("set generator row: %v", err)
	}

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性", "标签", "网格", "结算信息", "结算列表", "结算映射", "牌型"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs", "Tags", "Grid", "EndInfo", "EndInfos", "EndMap", "CType"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[int32]string", "map[string]int64", "[][]int64", "pb.TexasGameEndInfo", "[]pb.TexasGameEndInfo", "map[int32]pb.TexasGameEndInfo", "pb.CardType"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all", "all", "all", "all", "all", "all", "all"})
	// 数据行（严格按新分隔符）：
	//   Attrs:   map[int32]string, 元素;, K:V :
	//   Tags:    map[string]int64, 元素;
	//   Grid:    [][]int64, 1维|, 2维;
	//   EndInfo: pb.TexasGameEndInfo, 成员,（按字段顺序 uid,chair_id,chips,win_chips,card_type,hands,bests）
	//            hands/bests 是 repeated uint32，内部用 / 分隔（外部message专用repeated分隔符）
	//   EndInfos: []pb.TexasGameEndInfo, 数组元素|, 成员,
	//   EndMap:  map[int32]pb.TexasGameEndInfo, map元素;, K:V :, 成员,
	//   CType:   pb.CardType enum
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{
		"1",
		"1:hp;2:mp;3:atk",                    // map[int32]string
		"hp:100;mp:50",                       // map[string]int64
		"1|2;3|4",                            // [][]int64
		"1001,2,5000,1000,8,1/2/3/4,5/6/7/8", // pb.TexasGameEndInfo 单值(hands/bests用/)
		"1001,2,5000,1000,8,1/2/3/4,5/6/7/8|2002,3,6000,2000,9,8/7/6,5/4",     // []pb 数组(元素|)
		"1:1001,2,5000,1000,8,1/2/3/4,5/6/7/8;2:2002,3,6000,2000,9,8/7/6,5/4", // map[int32]pb(元素;)
		"8", // pb.CardType = FOUR_OF_A_KIND
	})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}

	// 外部 proto 源：指向 game_protocol（仓库根）
	protoSrcDir := filepath.Join(repoRoot(t), "game_protocol")
	protoText, jsonText, pbText := runIntegrationPipeline(t, xlsxPath, tmpDir, protoSrcDir)

	// ---- 1. proto 文本断言 ----
	checks := []string{
		`map<int32, string> Attrs`,
		`map<string, int64> Tags`,
		`TexasGameEndInfo EndInfo`,
		`repeated TexasGameEndInfo EndInfos`,
		`map<int32, TexasGameEndInfo> EndMap`,
		`CardType CType`,
		`import "proto/core/struct.proto"`,
		`import "proto/core/game_enum.proto"`,
	}
	for _, want := range checks {
		if !strings.Contains(protoText, want) {
			t.Errorf("proto 缺少 %q\n--- proto 片段 ---\n%s", want, protoText)
		}
	}
	// Grid 是 [][]int64，proto 里是 repeated PBARR_xxx Grid，只验证字段名存在
	if !strings.Contains(protoText, "Grid") {
		t.Errorf("proto 缺少 Grid 字段")
	}

	// ---- 2. JSON 断言 ----
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		t.Fatalf("unmarshal json: %v\n%s", err, jsonText)
	}
	if len(out) != 1 {
		t.Fatalf("行数应为1, got %d", len(out))
	}
	row := out[0]

	// map[int32]string -> {"1":"hp","2":"mp","3":"atk"}
	attrs, ok := row["Attrs"].(map[string]interface{})
	if !ok || attrs["1"] != "hp" || attrs["2"] != "mp" || attrs["3"] != "atk" {
		t.Errorf("Attrs 不符: %#v", row["Attrs"])
	}
	// map[string]int64 -> {"hp":100,"mp":50}
	tags, ok := row["Tags"].(map[string]interface{})
	if !ok || tags["hp"].(float64) != 100 || tags["mp"].(float64) != 50 {
		t.Errorf("Tags 不符: %#v", row["Tags"])
	}
	// [][]int64 -> [[1,2],[3,4]]
	grid, ok := row["Grid"].([]interface{})
	if !ok || len(grid) != 2 {
		t.Errorf("Grid 外层不符: %#v", row["Grid"])
	} else {
		g0 := grid[0].([]interface{})
		if g0[0].(float64) != 1 || g0[1].(float64) != 2 {
			t.Errorf("Grid[0] 不符: %#v", g0)
		}
	}
	// pb.TexasGameEndInfo 单值 -> {uid:1001,chair_id:2,chips:5000,win_chips:1000,card_type:8,hands:[1,2,3,4],bests:[5,6,7,8]}
	endInfo, ok := row["EndInfo"].(map[string]interface{})
	if !ok {
		t.Errorf("EndInfo 不符: %#v", row["EndInfo"])
	} else {
		if endInfo["uid"].(float64) != 1001 {
			t.Errorf("EndInfo.uid 不符: %#v", endInfo["uid"])
		}
		if endInfo["card_type"].(float64) != 8 {
			t.Errorf("EndInfo.card_type 不符: %#v", endInfo["card_type"])
		}
		hands := endInfo["hands"].([]interface{})
		if len(hands) != 4 || hands[0].(float64) != 1 {
			t.Errorf("EndInfo.hands 不符: %#v", endInfo["hands"])
		}
	}
	// []pb.TexasGameEndInfo -> 2元素
	endInfos, ok := row["EndInfos"].([]interface{})
	if !ok || len(endInfos) != 2 {
		t.Errorf("EndInfos 应2元素: %#v", row["EndInfos"])
	}
	// map[int32]pb.TexasGameEndInfo -> 2键
	endMap, ok := row["EndMap"].(map[string]interface{})
	if !ok || len(endMap) != 2 {
		t.Errorf("EndMap 应2键: %#v", row["EndMap"])
	} else {
		m1 := endMap["1"].(map[string]interface{})
		if m1["uid"].(float64) != 1001 {
			t.Errorf("EndMap[1].uid 不符: %#v", m1["uid"])
		}
	}
	// pb.CardType enum -> 数字8
	if row["CType"].(float64) != 8 {
		t.Errorf("CType 应为8: %#v", row["CType"])
	}

	// ---- 3. pb text (.conf) 断言 ----
	pbChecks := []string{
		`EndInfo: <`,  // 外部 message 字段渲染为嵌套 message
		`hands:`,      // repeated 标量字段
		`card_type:`,  // enum 字段
		`Attrs:`,      // map 字段
		`EndInfos: <`, // 外部 message 数组
	}
	for _, want := range pbChecks {
		if !strings.Contains(pbText, want) {
			t.Errorf("pb text 缺少 %q\n--- pb text 片段 ---\n%s", want, pbText)
		}
	}
}

// TestIntegration_EmptyValues 空值边界：map/外部message/多维数组 全空。
func TestIntegration_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "empty.xlsx")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	_ = file.SetSheetRow("生成表", "A1", &[]interface{}{"@config|Data:EmptyConfig"})

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "属性", "结算", "网格"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "Attrs", "EndInfo", "Grid"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "map[int32]string", "pb.TexasGameEndInfo", "[][]int64"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "all", "all", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{"1", "", "", ""}) // 全空

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	protoSrcDir := filepath.Join(repoRoot(t), "game_protocol")
	_, jsonText, _ := runIntegrationPipeline(t, xlsxPath, tmpDir, protoSrcDir)

	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("行数应为1, got %d", len(out))
	}
	row := out[0]
	// 空 map / 空 message / 空数组：JSON 里应为 nil 或不存在（不报错即通过）
	t.Logf("空值行 JSON: %#v", row)
	if row["Id"].(float64) != 1 {
		t.Errorf("Id 应为1: %#v", row["Id"])
	}
	// 空字段不应导致解析崩溃，且不产生错误数据
}

// TestIntegration_GenModeFilter gen 模式过滤：server 模式应过滤 client 字段。
func TestIntegration_GenModeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "genmode.xlsx")

	file := excelize.NewFile()
	defer file.Close()

	file.SetSheetName("Sheet1", "生成表")
	_ = file.SetSheetRow("生成表", "A1", &[]interface{}{"@config|Data:GenModeConfig"})

	file.NewSheet("Data")
	_ = file.SetSheetRow("Data", "A1", &[]interface{}{"编号", "服务器字段", "客户端字段", "全模式字段"})
	_ = file.SetSheetRow("Data", "A2", &[]interface{}{"Id", "ServerField", "ClientField", "AllField"})
	_ = file.SetSheetRow("Data", "A3", &[]interface{}{"int32", "int32", "string", "int32"})
	_ = file.SetSheetRow("Data", "A4", &[]interface{}{"all", "server", "client", "all"})
	_ = file.SetSheetRow("Data", "A5", &[]interface{}{"1", "100", "客户端文本", "200"})

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save: %v", err)
	}

	// server 模式
	fileName := strings.TrimSuffix(filepath.Base(xlsxPath), filepath.Ext(xlsxPath))
	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.JsonPath = filepath.Join(tmpDir, "json")
	domain.TextPath = ""
	domain.ProtoPath = ""
	domain.BytesPath = ""
	domain.LuaPath = ""
	domain.CodePath = ""
	domain.ConfMode = "server" // 关键：server 模式
	domain.PkgName = ""
	domain.PbPath = ""
	domain.ProtoSrcPath = ""

	resetGlobals(t)

	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if err := service.GenProto(); err != nil {
		t.Fatalf("GenProto: %v", err)
	}
	protoText := manager.GetProtoMap()[base.GetProtoName(fileName)]
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("ParseProto: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("GenData: %v", err)
	}

	// server 模式：ClientField 应被过滤，ServerField/AllField 保留
	if strings.Contains(protoText, "ClientField") {
		t.Errorf("server 模式不应生成 ClientField:\n%s", protoText)
	}
	if !strings.Contains(protoText, "ServerField") || !strings.Contains(protoText, "AllField") {
		t.Errorf("server 模式应保留 ServerField/AllField:\n%s", protoText)
	}

	jsonBytes, _ := os.ReadFile(filepath.Join(domain.JsonPath, "GenModeConfig.json"))
	var out []map[string]interface{}
	_ = json.Unmarshal(jsonBytes, &out)
	if len(out) == 1 {
		if _, exists := out[0]["ClientField"]; exists {
			t.Errorf("server 模式 JSON 不应含 ClientField: %#v", out[0])
		}
		if out[0]["ServerField"].(float64) != 100 {
			t.Errorf("ServerField 应为100: %#v", out[0]["ServerField"])
		}
	}
}
