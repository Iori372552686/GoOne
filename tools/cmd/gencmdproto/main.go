package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type cmdItem struct {
	name string
	val  int32
}

// Generates a proto enum that mirrors g1_protocol.CMD values for readability in IDL.
//
// Example:
//   go run ./tools/cmd/gencmdproto -prefix CMD_MAIN_ -prefix CMD_ROOM_CENTER_
//
// Notes:
// - Proto enums are int32; g1_protocol.CMD is also int32-based (protobuf-generated),
//   so values should fit.
func main() {
	var (
		outPath   = flag.String("out", filepath.ToSlash("api/proto/goone/cmd/v1/cmd.proto"), "output .proto path")
		pkg       = flag.String("pkg", "goone.cmd.v1", "proto package name")
		goPackage = flag.String("go_package", "github.com/Iori372552686/GoOne/api/gen/goone/cmd/v1;cmdv1", "option go_package")
		enumName  = flag.String("enum", "CMD", "enum name")
		hex       = flag.Bool("hex", true, "emit enum values as hex literals (0x...)")
		comments  = flag.Bool("comments", true, "try to copy comments from upstream g1_protocol Go sources")
		prefixes  = multiFlag{}
	)
	flag.Var(&prefixes, "prefix", "include only names with this prefix (repeatable). If empty, includes all CMDs.")
	flag.Parse()

	items := make([]cmdItem, 0, len(g1_protocol.CMD_name))
	for v, n := range g1_protocol.CMD_name {
		if len(prefixes) > 0 {
			ok := false
			for _, p := range prefixes {
				if strings.HasPrefix(n, p) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		items = append(items, cmdItem{name: n, val: v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].val != items[j].val {
			return items[i].val < items[j].val
		}
		return items[i].name < items[j].name
	})

	var commentByName map[string]string
	if *comments {
		commentByName = loadCmdComments()
	}

	out := renderCmdProto(*pkg, *goPackage, *enumName, *hex, items, commentByName)

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		die(err)
	}
	if err := os.WriteFile(*outPath, []byte(out), 0o644); err != nil {
		die(err)
	}
	fmt.Printf("[gencmdproto] wrote %s (%d entries)\n", *outPath, len(items))
}

// loadCmdComments tries to load comments for CMD_* from the upstream Go sources.
// This is best-effort: if we can't locate sources (no go toolchain, module cache not available),
// it returns an empty map.
func loadCmdComments() map[string]string {
	out := map[string]string{}

	// Find the module dir of github.com/Iori372552686/g1_common/protocol.
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "github.com/Iori372552686/g1_common/protocol")
	b, err := cmd.Output()
	if err != nil {
		return out
	}
	dir := strings.TrimSpace(string(b))
	if dir == "" {
		return out
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}

	// Match:
	//   // comment...
	//   CMD_XXX CMD = 0x...
	// Or:
	//   // comment...
	//   CMD_XXX CMD = 123
	re := regexp.MustCompile(`(?m)(?:^//[^\n]*\n)+^\s*(CMD_[A-Z0-9_]+)\s+CMD\s*=\s*([0-9A-Fa-fxX]+)`)
	reName := regexp.MustCompile(`(?m)^((?://[^\n]*\n)+)^\s*(CMD_[A-Z0-9_]+)\s+CMD\s*=\s*([0-9A-Fa-fxX]+)`)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		// Heuristic: only read files likely containing CMD enum.
		if !strings.Contains(name, "protocol") && !strings.Contains(name, "cmd") && !strings.Contains(name, "enum") {
			// still allow, but skip most files
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		// quick filter
		if !re.Match(data) {
			continue
		}
		matches := reName.FindAllSubmatch(data, -1)
		for _, m := range matches {
			if len(m) < 4 {
				continue
			}
			commentBlock := string(m[1])
			cmdName := string(m[2])
			// Keep the first comment we see for a given name (stable).
			if _, exists := out[cmdName]; exists {
				continue
			}
			out[cmdName] = strings.TrimRight(commentBlock, "\n")
		}
	}
	return out
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	*m = append(*m, v)
	return nil
}

func die(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "[gencmdproto] error:", err)
	os.Exit(1)
}


