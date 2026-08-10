package test

import (
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
	"github.com/xuri/excelize/v2"
)

// makeXLSX 构造一个最小 xlsx：可选生成表 + 数据 sheet 列表。
// genTableRows 为「生成表」sheet 的规则单元格（可为空，即无生成表）。
// dataSheets 为 map[sheet名]行内容（每行 []interface{} 同长度）。
func makeXLSX(t *testing.T, xlsxPath string, genTableRows []string, dataSheets map[string][][]interface{}) {
	t.Helper()
	file := excelize.NewFile()
	defer file.Close()

	if len(genTableRows) > 0 {
		file.SetSheetName("Sheet1", "生成表")
		for i, v := range genTableRows {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			if err := file.SetCellValue("生成表", cell, v); err != nil {
				t.Fatalf("set gen table row %d: %v", i, err)
			}
		}
	} else {
		file.SetSheetName("Sheet1", "占位")
	}

	for sheet, rows := range dataSheets {
		file.NewSheet(sheet)
		for r, row := range rows {
			if err := file.SetSheetRow(sheet, "A"+string(rune('1'+r)), &row); err != nil {
				t.Fatalf("set %s row %d: %v", sheet, r, err)
			}
		}
	}

	if err := file.SaveAs(xlsxPath); err != nil {
		t.Fatalf("save xlsx: %v", err)
	}
}

// runFullPipeline 跑完整 cfgtool 主流程（ParseFiles -> GenProto -> ParseProto -> GenData -> GenCode），
// 输出 proto 文本与生成的 Go 代码文件路径列表。
// outDir 为临时输出目录（json/text/code 都放其下）。
func runFullPipeline(t *testing.T, xlsxPath, outDir string) (protoText string, codeFiles []string) {
	t.Helper()
	fileName := filepath.Base(xlsxPath)
	feature := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	if i := indexByte(feature, '@'); i >= 0 {
		feature = feature[i+1:]
	}

	domain.XlsxPath = filepath.Dir(xlsxPath)
	domain.JsonPath = filepath.Join(outDir, "json")
	domain.TextPath = filepath.Join(outDir, "text")
	domain.ProtoPath = ""
	domain.CodePath = filepath.Join(outDir, "code")
	domain.ConfMode = "all"
	domain.Module = "github.com/Iori372552686/GoOne"
	domain.PbPath = "github.com/Iori372552686/g1_common/protocol"
	domain.PkgName = "g1_protocol"
	domain.ProtoSrcPath = ""
	resetGlobals(t)

	if err := parser.ParseFiles(xlsxPath); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if err := service.GenProto(); err != nil {
		t.Fatalf("GenProto: %v", err)
	}
	protoText = manager.GetProtoMap()[feature+".proto"]
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("ParseProto: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("GenData: %v", err)
	}
	if err := service.GenCode(); err != nil {
		t.Fatalf("GenCode: %v", err)
	}

	// 收集生成的 .go 文件
	codeRoot := domain.CodePath
	_ = filepath.Walk(codeRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			codeFiles = append(codeFiles, path)
		}
		return nil
	})
	return
}

// readGoFile 读取生成的 Go 代码文件内容。
func readGoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// gofmtValid 用 go/parser 校验生成的 Go 代码语法合法（不依赖外部 gofmt 命令）。
func gofmtValid(code string) error {
	_, err := goparser.ParseFile(token.NewFileSet(), "gen.go", code, goparser.AllErrors)
	return err
}
