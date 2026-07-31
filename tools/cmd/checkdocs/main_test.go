package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckDirDetectsBrokenAndValidLinks 验证 checkdocs 核心契约：
// 存在的相对链接不报、断链报；外部 http 链接与纯锚点不检查。
func TestCheckDirDetectsBrokenAndValidLinks(t *testing.T) {
	dir := t.TempDir()

	// 创建一个被引用的目标文件。
	target := filepath.Join(dir, "target.md")
	if err := os.WriteFile(target, []byte("# target\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 含 1 个有效链接、1 个断链、1 个外部链接、1 个纯锚点的 md。
	src := strings.Join([]string{
		"[good](target.md)",
		"[bad](missing.md)",
		"[ext](https://example.com/x)",
		"[anchor](#section)",
		"<relative.md>",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	broken, err := checkDir(dir)
	if err != nil {
		t.Fatalf("checkDir: %v", err)
	}

	// 期望断链：missing.md 与 relative.md。
	got := map[string]bool{}
	for _, b := range broken {
		got[b.target] = true
	}
	if !got["missing.md"] {
		t.Errorf("expected missing.md reported broken; got %v", broken)
	}
	if !got["relative.md"] {
		t.Errorf("expected relative.md reported broken; got %v", broken)
	}
	if got["target.md"] {
		t.Errorf("existing target.md must not be reported; got %v", broken)
	}
	if got["https://example.com/x"] {
		t.Errorf("external link must be skipped; got %v", broken)
	}
}

// TestCheckDirRecursesIntoSubdirs 验证递归扫描子目录。
func TestCheckDirRecursesIntoSubdirs(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// 子目录文件引用上级存在的文件。
	if err := os.WriteFile(filepath.Join(dir, "up.md"), []byte("# up\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "child.md"), []byte("[up](../up.md)\n[bad](../nope.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	broken, err := checkDir(dir)
	if err != nil {
		t.Fatalf("checkDir: %v", err)
	}
	found := false
	for _, b := range broken {
		if b.target == "../nope.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ../nope.md broken in subdir; got %v", broken)
	}
}

// TestExtractLinksIgnoresInlineCode 验证反引号包裹的行内代码（如 `<Service>`、
// `[x](y)`）不被误判为链接。
func TestExtractLinksIgnoresInlineCode(t *testing.T) {
	dir := t.TempDir()
	// 真实链接在代码外，伪链接在反引号内。
	src := "see [real](real.md); code uses `<Service>SS` and `[fake](nope.md)`\n"
	if err := os.WriteFile(filepath.Join(dir, "real.md"), []byte("# real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	broken, err := checkDir(dir)
	if err != nil {
		t.Fatalf("checkDir: %v", err)
	}
	for _, b := range broken {
		if b.target == "nope.md" || b.target == "Service" {
			t.Fatalf("inline-code link should be ignored; got broken=%v", b)
		}
	}
}

// TestRunExitCodes 验证 run 的退出码语义：无断链返回 0，有断链返回 1。
func TestRunExitCodes(t *testing.T) {
	dir := t.TempDir()
	// 空 docs（无断链）。
	if code := run([]string{dir}, os.Stdout, os.Stderr); code != 0 {
		t.Fatalf("empty docs: expected exit 0, got %d", code)
	}
	// 引入断链。
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("[x](no.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{dir}, os.Stdout, os.Stderr); code != 1 {
		t.Fatalf("broken docs: expected exit 1, got %d", code)
	}
}
