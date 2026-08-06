package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func main() {
	var (
		module    = flag.String("module", "github.com/Iori372552686/GoOne", "Go module path (used for output layout)")
		outDir    = flag.String("out", ".", "output directory (typically repo root)")
		protoRoot = flag.String("proto_root", "api/proto", "proto root directory")
		protoc    = flag.String("protoc", "", "path to protoc (optional). If empty, tries PATH then repo-vendored (linux only).")
	)
	flag.Parse()

	repoRoot, err := findRepoRoot()
	if err != nil {
		die(err)
	}

	absOut := filepath.Join(repoRoot, *outDir)
	absProtoRoot := filepath.Join(repoRoot, *protoRoot)
	gameProtocolRoot := filepath.Join(repoRoot, "game_protocol")

	// Ensure cmd.proto exists (new checkouts shouldn't fail to missing generated IDL).
	if err := ensureCmdProto(repoRoot, absProtoRoot); err != nil {
		die(err)
	}

	protocPath, err := findProtoc(repoRoot, *protoc)
	if err != nil {
		die(err)
	}

	binDir := filepath.Join(repoRoot, ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		die(err)
	}

	protocGenGo := filepath.Join(binDir, exeName("protoc-gen-go"))
	protocGenGoone := filepath.Join(binDir, exeName("protoc-gen-goone"))

	// build plugins (deterministic, pinned to repo go.mod)
	if err := goBuild(repoRoot, protocGenGoone, "./tools/protoc-gen-goone"); err != nil {
		die(err)
	}
	if err := goBuild(repoRoot, protocGenGo, "google.golang.org/protobuf/cmd/protoc-gen-go"); err != nil {
		die(err)
	}

	// proto inputs are split into two protoc invocations:
	// 1) repo-owned api/proto/** inputs (legacy + current main-repo service protos)
	// 2) protocol-owned service protos from game_protocol/** that declare services
	//
	// We must keep them separate because repo-owned service protos still import the
	// temporary third_party stubs, while protocol-owned service protos import the
	// real game_protocol messages. Mixing both worlds in one protoc invocation
	// causes duplicate symbol errors for g1.protocol message types.
	repoInputs, protocolInputs, err := collectProtoInputs(absProtoRoot, gameProtocolRoot)
	if err != nil {
		die(err)
	}
	if len(repoInputs) == 0 && len(protocolInputs) == 0 {
		die(errors.New("no proto files found under api/proto/{goone,game,web} or game_protocol service protos"))
	}

	// Inputs are already collected relative to their proto roots and must stay that
	// way so protoc can resolve them under the matching -I include path.
	for i := range repoInputs {
		repoInputs[i] = filepath.ToSlash(repoInputs[i])
	}
	for i := range protocolInputs {
		protocolInputs[i] = filepath.ToSlash(protocolInputs[i])
	}

	googleIncludes := existingGoogleIncludes(repoRoot)

	if len(repoInputs) > 0 {
		repoIncludePaths := []string{absProtoRoot}
		repoIncludePaths = append(repoIncludePaths, googleIncludes...)
		if err := runProtoc(protocPath, repoRoot, absOut, *module, protocGenGo, protocGenGoone, repoIncludePaths, repoInputs, "repo"); err != nil {
			die(err)
		}
	}

	if len(protocolInputs) > 0 {
		protocolIncludePaths := []string{gameProtocolRoot, absProtoRoot}
		protocolIncludePaths = append(protocolIncludePaths, googleIncludes...)
		if err := runProtoc(protocPath, repoRoot, absOut, *module, protocGenGo, protocGenGoone, protocolIncludePaths, protocolInputs, "protocol"); err != nil {
			die(err)
		}
	}

	fmt.Println("[genproto] done")
}

func die(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "[genproto] error:", err)
	os.Exit(1)
}

func ensureCmdProto(repoRoot, absProtoRoot string) error {
	cmdProto := filepath.Join(absProtoRoot, "goone", "cmd", "v1", "cmd.proto")
	_, statErr := os.Stat(cmdProto)
	if statErr == nil && os.Getenv("GEN_CMD_PROTO") != "1" {
		return nil
	}
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) && os.Getenv("GEN_CMD_PROTO") != "1" {
		return fmt.Errorf("stat cmd.proto: %w", statErr)
	}

	// Generate via go run so this works cross-platform (no shell script dependency).
	fmt.Printf("[genproto] ensure cmd.proto -> %s\n", cmdProto)
	cmd := exec.Command("go", "run", "./tools/cmd/gencmdproto")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate cmd.proto (gencmdproto): %w", err)
	}
	return nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", errors.New("cannot locate repo root (go.mod not found)")
}

func goBuild(repoRoot, out, pkg string) error {
	args := []string{"build", "-o", out, pkg}
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findProtoc(repoRoot string, override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("protoc not found at %q: %w", override, err)
		}
		return override, nil
	}
	if p, err := exec.LookPath(exeName("protoc")); err == nil {
		return p, nil
	}
	for _, p := range vendoredProtocCandidates(repoRoot) {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("protoc not found on PATH or in vendored locations under lib/contrib/protoc or lib/contrib/bundled/protoc_legacy")
}

func existingGoogleIncludes(repoRoot string) []string {
	candidates := vendoredIncludeCandidates(repoRoot)
	out := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

func vendoredProtocCandidates(repoRoot string) []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-win64", "bin", "protoc.exe"),
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-win64", "bin", "protoc.exe"),
			filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-win64", "bin", "protoc.exe"),
		}
	case "darwin":
		return []string{
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-osx-aarch_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-osx-aarch_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-osx-x86_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-osx-x86_64", "bin", "protoc"),
		}
	default:
		return []string{
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-linux-x86_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-linux-x86_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-33.2-linux-x86_64", "bin", "protoc"),
			filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-linux-x86_64", "bin", "protoc"),
		}
	}
}

func vendoredIncludeCandidates(repoRoot string) []string {
	return []string{
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-win64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-win64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-linux-x86_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-linux-x86_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-33.2-osx-aarch_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-osx-aarch_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "protoc", "protoc-30.1-osx-x86_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-33.2-win64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-win64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-33.2-linux-x86_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-linux-x86_64", "include"),
		filepath.Join(repoRoot, "lib", "contrib", "bundled", "protoc_legacy", "protoc-3.11.4-osx-x86_64", "include"),
	}
}

var serviceDeclRE = regexp.MustCompile(`(?m)^\s*service\s+[A-Za-z_][A-Za-z0-9_]*\s*\{`)

func collectProtoInputs(absProtoRoot, gameProtocolRoot string) ([]string, []string, error) {
	roots := []string{
		filepath.Join(absProtoRoot, "goone"),
		filepath.Join(absProtoRoot, "game"),
	}
	webRoot := filepath.Join(absProtoRoot, "web")
	if st, err := os.Stat(webRoot); err == nil && st.IsDir() {
		roots = append(roots, webRoot)
	}
	var repoProtos []string
	for _, r := range roots {
		_ = filepath.WalkDir(r, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(d.Name()), ".proto") {
				// protoc wants paths relative to proto_root for clean import mapping
				rel, err := filepath.Rel(absProtoRoot, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				repoProtos = append(repoProtos, rel)
			}
			return nil
		})
	}
	var protocolProtos []string
	if st, err := os.Stat(gameProtocolRoot); err == nil && st.IsDir() {
		_ = filepath.WalkDir(gameProtocolRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".proto") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if !serviceDeclRE.Match(data) {
				return nil
			}
			rel, relErr := filepath.Rel(gameProtocolRoot, path)
			if relErr != nil {
				return relErr
			}
			protocolProtos = append(protocolProtos, filepath.ToSlash(rel))
			return nil
		})
	}
	sort.Strings(repoProtos)
	sort.Strings(protocolProtos)
	return repoProtos, protocolProtos, nil
}

func runProtoc(
	protocPath string,
	repoRoot string,
	outDir string,
	module string,
	protocGenGo string,
	protocGenGoone string,
	includePaths []string,
	inputs []string,
	label string,
) error {
	args := make([]string, 0, len(includePaths)*2+len(inputs)+8)
	for _, inc := range includePaths {
		args = append(args, "-I", inc)
	}
	args = append(args,
		fmt.Sprintf("--plugin=protoc-gen-go=%s", protocGenGo),
		fmt.Sprintf("--plugin=protoc-gen-goone=%s", protocGenGoone),
		fmt.Sprintf("--go_out=%s", outDir),
		fmt.Sprintf("--go_opt=module=%s", module),
		"--go_opt=paths=import",
		fmt.Sprintf("--goone_out=%s", outDir),
		fmt.Sprintf("--goone_opt=module=%s", module),
		"--goone_opt=paths=import",
	)
	args = append(args, inputs...)

	cmd := exec.Command(protocPath, args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("[genproto] protoc=%s\n", protocPath)
	fmt.Printf("[genproto] module=%s\n", module)
	fmt.Printf("[genproto] %s_inputs=%d\n", label, len(inputs))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("protoc failed for %s inputs: %w", label, err)
	}
	return nil
}
