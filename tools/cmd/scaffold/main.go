// Package main implements a lightweight scaffold tool for creating new GoOne server skeletons.
//
// Usage:
//
//	go run tools/cmd/scaffold -name mysvr
//	go run tools/cmd/scaffold -name mysvr -module github.com/Iori372552686/GoOne -out src/
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

func main() {
	var (
		name   = flag.String("name", "", "server name (required, e.g. mysvr)")
		module = flag.String("module", "github.com/Iori372552686/GoOne", "Go module path")
		out    = flag.String("out", "src", "output parent directory (server dir is created inside)")
	)
	flag.Parse()

	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: -name is required")
		flag.Usage()
		os.Exit(1)
	}

	svrName := strings.ToLower(strings.TrimSpace(*name))
	if !strings.HasSuffix(svrName, "svr") {
		svrName += "svr"
	}

	data := templateData{
		Name:       svrName,
		StructName: toPascalCase(svrName),
		ImplName:   toPascalCase(svrName) + "Impl",
		Module:     *module,
	}

	svrDir := filepath.Join(*out, svrName)
	if err := generateAll(svrDir, data); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s/\n", svrDir)
	fmt.Println("Next steps:")
	fmt.Printf("  1. Add config in common/gconf/config.go:  type %s struct { SelfBusId, LogDir, LogLevel string }\n", data.StructName)
	fmt.Printf("  2. Add config YAML section for %s\n", svrName)
	fmt.Printf("  3. Define proto service in api/proto/game/%s/v1/\n", strings.TrimSuffix(svrName, "svr"))
	fmt.Printf("  4. Run: go build ./src/%s/\n", svrName)
}

type templateData struct {
	Name       string // e.g. "mysvr"
	StructName string // e.g. "Mysvr"
	ImplName   string // e.g. "MysvrImpl"
	Module     string // e.g. "github.com/Iori372552686/GoOne"
}

func generateAll(svrDir string, data templateData) error {
	files := []struct {
		relPath  string
		template string
	}{
		{"main.go", tplMainGo},
		{"app.go", tplAppGo},
		{filepath.Join("globals", "globals.go"), tplGlobalsGo},
		{filepath.Join("cmd_handler", "register.go"), tplRegisterGo},
	}

	for _, f := range files {
		outPath := filepath.Join(svrDir, f.relPath)
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
// Templates
// ---------------------------------------------------------------------------

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

	if err := newApp().Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

var tplAppGo = `package {{.Name}}

import (
	"context"
	"fmt"

	"{{.Module}}/common/gconf"
	"{{.Module}}/lib/api/logger"
	"{{.Module}}/lib/service/runtime"
	"{{.Module}}/lib/service/runtime/bussvc"
	"{{.Module}}/module/misc"
	"{{.Module}}/src/{{.Name}}/globals"
)

// NewApp 用 runtime.App + Component 装配 {{.Name}}（scaffold 生成的最小骨架）。
func NewApp() *runtime.App {
	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}
	routerComp := &bussvc.RouterComponent{
		Common:   svcCommon,
		TransMgr: globals.TransMgr,
	}

	app, err := runtime.New("{{.Name}}",
		runtime.WithLoadConfig(func(_ context.Context) error {
			// TODO: 更新为加载你的配置 struct。
			// return gconf.Load{{.StructName}}Config(*gconf.SvrConfFile)
			_ = gconf.SvrConfFile
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("runtime.New {{.Name}}: %v", err))
	}

	// Start 顺序：TransMgr → router/bus。逆序用于 Quiesce/Drain/Stop。
	for _, c := range []runtime.Component{transMgr, routerComp} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("{{.Name}} register %s: %v", c.Name(), err))
		}
	}
	logger.Infof("================== {{.Name}} scaffold app built =========================")
	return app
}

// svcCommon 从 gconf 产出 {{.Name}} 的 bus 服务共享配置段。
// TODO: 在加入配置 struct 后，填充以下字段。
func svcCommon() bussvc.Common {
	return bussvc.Common{}
}
`

var tplGlobalsGo = `package globals

import (
	"{{.Module}}/lib/service/transaction"
)

var (
	TransMgr = transaction.NewTransactionMgr()
	// TODO: add domain-specific managers here, e.g.:
	// RedisMgr = redis.NewRedisMgr()
)
`

var tplRegisterGo = `package cmd_handler

import (
	"{{.Module}}/lib/api/logger"
	// "{{.Module}}/src/{{.Name}}/globals"
	// g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// RegCmd registers all command handlers for {{.Name}}.
func RegCmd() {
	logger.Infof("register transaction commands")
	// If this service is IDL-first, generated ssrpc registration will usually be
	// wired from app.go instead, and this file can stay empty or be removed later.
	// TODO: register your cmd handlers, e.g.:
	// globals.TransMgr.RegisterCmd(g1_protocol.CMD_XXX_REQ, YourHandler)
}
`
