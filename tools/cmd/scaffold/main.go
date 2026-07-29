// Package main implements a lightweight scaffold tool for creating new GoOne server skeletons.
//
// Usage:
//
//	go run tools/cmd/scaffold -name mysvr
//	go run tools/cmd/scaffold -name mysvr -root . -module github.com/Iori372552686/GoOne
//
// P1-08：命令逻辑拆为可测试的 run(args, stdout, stderr) error；main 只负责 exit code。
// 默认在仓库根目录下生成标准结构 cmd/<service>/main.go 与 src/<service>/app.go；main.go
// 调用 NewApp()（与 app.go 一致）。最小 app 只调用 runtime.MustNew("<service>")，不伪
// 造尚不存在的配置/bus/Handler/manager。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 解析 args 并生成脚手架。stdout/stderr 注入便于测试。返回进程 exit code。
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "server name (required, e.g. mysvr)")
	module := fs.String("module", defaultModulePath, "Go module path")
	root := fs.String("root", ".", "repository root directory (cmd/<service> and src/<service> are created under it)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := generate(*name, *module, *root, stdout); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// defaultModulePath 从当前 go.mod 读取 module path；失败回退到仓库默认。
var defaultModulePath = "github.com/Iori372552686/GoOne"

// generate 是脚手架的核心：校验名称、解析 module、在 root 下生成 cmd/<service> 与
// src/<service>。
func generate(name, module, root string, stdout io.Writer) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("-name is required")
	}
	if err := validateServiceName(name); err != nil {
		return err
	}
	svrName := name
	if !strings.HasSuffix(svrName, "svr") {
		svrName += "svr"
	}
	if strings.TrimSpace(module) == "" {
		return fmt.Errorf("-module is required (could not read go.mod)")
	}

	data := templateData{
		Name:       svrName,
		StructName: toPascalCase(svrName),
		ImplName:   toPascalCase(svrName) + "Impl",
		Module:     module,
	}

	if err := generateAll(root, data); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created cmd/%s/main.go and src/%s/app.go under %s\n", svrName, svrName, root)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintf(stdout, "  1. Add a config struct + LoadConfig in common/gconf/config.go for %s\n", svrName)
	fmt.Fprintf(stdout, "  2. Add a config YAML section for %s\n", svrName)
	fmt.Fprintf(stdout, "  3. Define proto service in api/proto/game/%s/v1/ and regenerate api/gen\n", strings.TrimSuffix(svrName, "svr"))
	fmt.Fprintf(stdout, "  4. go test ./src/%s && go build ./cmd/%s\n", svrName, svrName)
	return nil
}

// validateServiceName 拒绝非法服务名（含路径分隔符、空段、非 ASCII 等）。
func validateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name is empty")
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r == filepath.Separator || unicode.IsSpace(r) {
			return fmt.Errorf("service name %q contains invalid character %q", name, r)
		}
	}
	return nil
}

type templateData struct {
	Name       string // e.g. "mysvr"
	StructName string // e.g. "Mysvr"
	ImplName   string // e.g. "MysvrImpl"
	Module     string // e.g. "github.com/Iori372552686/GoOne"
}

// generateAll 在 root 下生成 cmd/<service>/main.go 与 src/<service>/app.go。
func generateAll(root string, data templateData) error {
	files := []struct {
		relPath  string
		template string
	}{
		{filepath.Join("cmd", data.Name, "main.go"), tplMainGo},
		{filepath.Join("src", data.Name, "app.go"), tplAppGo},
	}

	for _, f := range files {
		outPath := filepath.Join(root, f.relPath)
		if _, err := os.Stat(outPath); err == nil {
			return fmt.Errorf("file already exists: %s (refusing to overwrite)", outPath)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		tmpl, err := template.New(f.relPath).Parse(f.template)
		if err != nil {
			return fmt.Errorf("template parse %s: %w", f.relPath, err)
		}
		w, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(w, data); err != nil {
			w.Close()
			return fmt.Errorf("template exec %s: %w", f.relPath, err)
		}
		w.Close()
	}
	return nil
}

func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ---------------------------------------------------------------------------
// Templates (P1-08)
// ---------------------------------------------------------------------------

// tplMainGo 生成 cmd/<service>/main.go，调用 NewApp()（与 app.go 一致）。
var tplMainGo = `package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"{{.Module}}/lib/api/logger"
	"{{.Module}}/src/{{.Name}}"
)

func main() {
	flag.Parse()
	defer logger.Flush()

	if err := {{.Name}}.NewApp().Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

// tplAppGo 生成 src/<service>/app.go。最小骨架：runtime.MustNew 到 Ready 并等待关停信号，
// 不监听业务端口、不伪造配置/bus/Handler/manager。增加真实组件后按 STYLE 用
// app.MustRegister(...)。
var tplAppGo = `package {{.Name}}

import (
	"github.com/Iori372552686/GoOne/lib/service/runtime"
)

// NewApp 装配 {{.Name}} 的最小 runtime.App。
//
// scaffold 生成的最小进程：启动到 Ready 并等待关停信号，不监听业务端口。增加真实组件
// （logger/admin/tracing/dependencies/ssrpc_registry/transaction_mgr/router/domain）后，
// 用 app.MustRegister(...) 按序注册。
func NewApp() *runtime.App {
	app := runtime.MustNew("{{.Name}}")
	// TODO: 增加组件后在此 MustRegister，例如：
	// app.MustRegister(scheduler.DefaultDateTimeTick(), logComp, adminComp, ...)
	return app
}
`
