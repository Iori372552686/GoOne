// Command checkmodules verifies GoOne's library modules against the current checkout.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type manifest struct {
	Module  struct{ Path string }
	Replace []json.RawMessage
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: checkmodules check|test|race|integration|security [module-directory ...]")
	}

	var checks [][]string
	switch os.Args[1] {
	case "check":
		checks = [][]string{
			{"go", "build", "./..."},
			{"go", "vet", "./..."},
			{"go", "test", "-count=1", "-timeout=600s", "./..."},
		}
	case "test":
		checks = [][]string{{"go", "test", "-count=1", "-timeout=600s", "./..."}}
	case "race":
		checks = [][]string{{"go", "test", "-race", "-count=1", "-timeout=600s", "./..."}}
	case "integration":
		checks = [][]string{{"go", "test", "-v", "-count=1", "-timeout=600s", "./..."}}
	case "security":
		command := []string{"govulncheck"}
		if db := os.Getenv("GOONE_GOVULN_DB"); db != "" {
			command = append(command, "-db", db)
		}
		checks = [][]string{append(command, "./...")}
	default:
		return fmt.Errorf("unknown check %q", os.Args[1])
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}
	rootManifest, err := readManifest(root)
	if err != nil {
		return err
	}
	rootModule := rootManifest.Module.Path

	var dirs []string
	err = filepath.WalkDir("lib", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "go.mod" {
			dirs = append(dirs, filepath.ToSlash(filepath.Dir(path)))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		return errors.New("no library modules found, run from the GoOne repository root")
	}

	selected := make(map[string]bool)
	for _, arg := range os.Args[2:] {
		dir := filepath.ToSlash(filepath.Clean(arg))
		found := false
		for _, candidate := range dirs {
			if candidate == dir || strings.HasPrefix(candidate, dir+"/") {
				selected[candidate] = true
				found = true
			}
		}
		if !found {
			return fmt.Errorf("no library modules under %q", arg)
		}
	}

	// Only the temporary main module supplies replacements. The checked-in
	// library manifests use versioned dependencies and can be downloaded by Go.
	replacements := []string{"-replace=github.com/Iori372552686/g1_common=" + filepath.Join(root, "common")}
	for _, dir := range dirs {
		replacements = append(replacements,
			"-replace="+rootModule+"/"+dir+"="+filepath.Join(root, filepath.FromSlash(dir)))
	}

	temp, err := os.MkdirTemp("", "goone-checkmodules-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temp) }()

	modfile := filepath.Join(temp, "check.mod")
	sumfile := filepath.Join(temp, "check.sum")
	for _, dir := range dirs {
		if len(selected) > 0 && !selected[dir] {
			continue
		}
		fmt.Printf("==> %s\n", dir)
		original, err := readManifest(dir)
		if err != nil {
			return err
		}
		if original.Module.Path != rootModule+"/"+dir || len(original.Replace) != 0 {
			return fmt.Errorf("%s must use its repository path and versioned dependencies", dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
			return err
		}
		if err := copyManifest(modfile, filepath.Join(dir, "go.mod"), false); err != nil {
			return err
		}
		if err := copyManifest(sumfile, filepath.Join(dir, "go.sum"), true); err != nil {
			return err
		}
		cmd := exec.Command("go", append(append([]string{"mod", "edit"}, replacements...), modfile)...)
		cmd.Env = append(os.Environ(), "GOWORK=off")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: prepare local dependencies: %w", dir, err)
		}

		if os.Args[1] == "check" {
			if err := checkGraph(dir, modfile, rootModule); err != nil {
				return err
			}
		}
		for _, check := range checks {
			cmd := inModule(dir, modfile, check...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: %s: %w", dir, strings.Join(check, " "), err)
			}
		}
	}
	return nil
}

func readManifest(dir string) (manifest, error) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	raw, err := cmd.Output()
	if err != nil {
		return manifest{}, fmt.Errorf("%s: read go.mod: %w", dir, err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, fmt.Errorf("%s: decode go.mod: %w", dir, err)
	}
	return m, nil
}

func copyManifest(dst, src string, optional bool) error {
	raw, err := os.ReadFile(src)
	if err != nil && (!optional || !errors.Is(err, fs.ErrNotExist)) {
		return err
	}
	return os.WriteFile(dst, raw, 0600)
}

func inModule(dir, modfile string, command ...string) *exec.Cmd {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	flags := strings.TrimSpace(os.Getenv("GOFLAGS") + " -mod=readonly '-modfile=" + filepath.ToSlash(modfile) + "'")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS="+flags)
	return cmd
}

func checkGraph(dir, modfile, rootModule string) error {
	cmd := inModule(dir, modfile, "go", "list", "-m", "-f", "{{.Path}}", "all")
	cmd.Stderr = os.Stderr
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: read module graph: %w", dir, err)
	}

	mqSDK := map[string]string{
		"github.com/apache/rocketmq-client-go/v2": "rocketmq",
		"github.com/nats-io/nats.go":              "nats",
		"github.com/nsqio/go-nsq":                 "nsq",
		"github.com/rabbitmq/amqp091-go":          "rabbitmq",
		"github.com/segmentio/kafka-go":           "kafka",
	}
	const driverPrefix = "lib/service/bus/driver/"
	for dependency := range strings.FieldsSeq(string(raw)) {
		if dependency == rootModule {
			return fmt.Errorf("%s unexpectedly requires the GoOne root module", dir)
		}
		if backend, ok := mqSDK[dependency]; ok {
			switch {
			case dir == "lib/service/bus":
				return fmt.Errorf("%s unexpectedly requires %s", dir, dependency)
			case strings.HasPrefix(dir, driverPrefix) && dir != driverPrefix+backend:
				return fmt.Errorf("%s unexpectedly requires %s", dir, dependency)
			}
		}
	}
	return nil
}
