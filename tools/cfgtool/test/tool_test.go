package test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
)

// repoRoot 解析到 GoOne 仓库根目录，使本测试不依赖当前工作目录。
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// tools/cfgtool/test/tool_test.go -> 仓库根上溯 4 级
	return filepath.Clean(filepath.Join(thisFile, "..", "..", "..", ".."))
}

// resetGlobals 把 domain 全局变量恢复到初始零值，避免多个测试间互相污染。
// multi_array_test 也改用此 helper，二者可在 -parallel 下安全共存。
func resetGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		manager.Clear()
		domain.Module = ""
		domain.ConfMode = ""
		domain.PkgName = ""
		domain.XlsxPath = ""
		domain.ProtoPath = ""
		domain.PbPath = ""
		domain.CodePath = ""
		domain.CppPath = ""
		domain.NodeJsPath = ""
		domain.JsonPath = ""
		domain.BytesPath = ""
		domain.TextPath = ""
		domain.LuaPath = ""
		domain.TsPath = ""
	})
}

func TestConfig(t *testing.T) {
	resetGlobals(t)
	root := repoRoot(t)

	domain.XlsxPath = filepath.Join(root, "tools", "cfgtool", "xls")
	domain.JsonPath = filepath.Join(root, "tools", "cfgtool", "gen", "json")
	domain.ProtoPath = filepath.Join(root, "tools", "cfgtool", "gen", "proto")
	domain.CodePath = filepath.Join(root, "tools", "cfgtool", "gen", "code")
	domain.LuaPath = filepath.Join(root, "tools", "cfgtool", "gen", "lua")
	domain.Module = "github.com/Iori372552686/GoOne"
	domain.PbPath = "github.com/Iori372552686/game_protocol/protocol"
	domain.PkgName = filepath.Base(domain.PbPath)

	// 加载所有配置
	files, err := base.Glob(domain.XlsxPath, ".*\\.xlsx", true)
	if err != nil {
		t.Fatalf("glob xlsx: %v", err)
	}
	if len(files) == 0 {
		t.Skipf("no xlsx fixtures under %s; skipping", domain.XlsxPath)
	}

	// 解析所有文件
	if err := parser.ParseFiles(files...); err != nil {
		t.Fatalf("parse files: %v", err)
	}
	// 生成proto文件数据
	if err := service.GenProto(); err != nil {
		t.Fatalf("gen proto: %v", err)
	}
	if err := service.SaveProto(); err != nil {
		t.Fatalf("save proto: %v", err)
	}
	// 解析proto文件
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("parse proto: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("gen data: %v", err)
	}
	if err := service.GenCode(); err != nil {
		t.Fatalf("gen code: %v", err)
	}
}
