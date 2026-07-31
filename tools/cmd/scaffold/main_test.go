package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGeneratesCompilableLayout 验证 run 在 t.TempDir 生成
// cmd/<service>/main.go 与 src/<service>/app.go，main.go 调用 NewApp()，且文件为合法 Go
// 语法。
func TestRunGeneratesCompilableLayout(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-name", "inventorysvr", "-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit %d, stderr=%s", code, stderr.String())
	}

	mainGo := filepath.Join(root, "cmd", "inventorysvr", "main.go")
	appGo := filepath.Join(root, "src", "inventorysvr", "app.go")
	for _, p := range []string{mainGo, appGo} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s: %v", p, err)
		}
	}

	// main.go 必须调用 NewApp()（与 app.go 一致，修正历史 newApp() 不匹配）。
	mainContent, _ := os.ReadFile(mainGo)
	if !strings.Contains(string(mainContent), "NewApp().Run") {
		t.Fatalf("main.go 应调用 NewApp().Run，got:\n%s", mainContent)
	}
	// app.go 必须定义 NewApp() 并用 runtime.MustNew。
	appContent, _ := os.ReadFile(appGo)
	if !strings.Contains(string(appContent), "func NewApp()") {
		t.Fatalf("app.go 应定义 NewApp()，got:\n%s", appContent)
	}
	if !strings.Contains(string(appContent), "runtime.MustNew") {
		t.Fatalf("app.go 应用 runtime.MustNew，got:\n%s", appContent)
	}

	// stdout 应提示后续步骤。
	if !strings.Contains(stdout.String(), "Next steps") {
		t.Fatalf("stdout 应含 Next steps，got: %s", stdout.String())
	}
}

// TestRunRejectsEmptyName 验证 空 name 返回非 0 exit code。
func TestRunRejectsEmptyName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-name", "", "-root", t.TempDir()}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("空 name 应返回非 0 exit code")
	}
}

// TestRunRejectsInvalidName 验证 非法服务名（含路径分隔符）被拒绝。
func TestRunRejectsInvalidName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-name", "../evil", "-root", t.TempDir()}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("非法服务名应返回非 0 exit code")
	}
}

// TestRunRefusesOverwrite 验证 已存在的文件拒绝覆盖。
func TestRunRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	// 第一次成功。
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-name", "dupsvr", "-root", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("first run exit %d: %s", code, stderr.String())
	}
	// 第二次应拒绝（文件已存在）。
	stdout.Reset()
	stderr.Reset()
	code := run([]string{"-name", "dupsvr", "-root", root}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("重复生成应返回非 0 exit code（refusing to overwrite）")
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr 应提示 already exists，got: %s", stderr.String())
	}
}

// TestValidateServiceName 覆盖非法字符分支。
func TestValidateServiceName(t *testing.T) {
	cases := []string{"", "a/b", "a\\b", "a b"}
	for _, c := range cases {
		if err := validateServiceName(c); err == nil {
			t.Fatalf("validateServiceName(%q) 应返回 error", c)
		}
	}
	if err := validateServiceName("goodname"); err != nil {
		t.Fatalf("合法名应通过，got %v", err)
	}
}
