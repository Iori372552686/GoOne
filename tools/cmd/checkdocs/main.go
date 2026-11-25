// Command checkdocs 递归扫描 docs/ 目录下所有 Markdown 文件，校验其中的相对链接
// （[text](path) 与 <path> 形式）指向的本地文件确实存在。外部 http(s) 链接、锚点
// （#frag）与 mailto: 不检查。返回非 0 退出码表示发现断链。
//
// 用法：
//
//	go run ./tools/cmd/checkdocs ./docs
//
// 设计目标（CI docs 门禁）：取代仅扫描 README/CHANGELOG 的旧逻辑，覆盖整个 docs/ 目录，
// 使文档迁移后内部断链能被 CI 拦截。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 是可测试的命令主体，返回进程退出码。root 为待扫描的 docs 目录。
func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("checkdocs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}

	broken, err := checkDir(root)
	if err != nil {
		fmt.Fprintf(stderr, "checkdocs: %v\n", err)
		return 2
	}
	for _, b := range broken {
		fmt.Fprintf(stdout, "broken link: %s -> %s\n", b.file, b.target)
	}
	if len(broken) > 0 {
		fmt.Fprintf(stderr, "checkdocs: %d broken link(s) found\n", len(broken))
		return 1
	}
	fmt.Fprintf(stdout, "checkdocs: no broken links under %s\n", root)
	return 0
}

type brokenLink struct {
	file   string // 含链接的 .md 文件（相对 root）
	target string // 断链目标
}

// checkDir 递归扫描 root 下所有 .md 文件，返回断链列表。
func checkDir(root string) ([]brokenLink, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var broken []brokenLink
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		links, lerr := extractLinks(path)
		if lerr != nil {
			return lerr
		}
		for _, link := range links {
			if !isLocalLink(link) {
				continue
			}
			target := cleanTarget(link)
			if target == "" {
				continue
			}
			abs := target
			if !filepath.IsAbs(target) {
				abs = filepath.Join(filepath.Dir(path), target)
			}
			abs = filepath.Clean(abs)
			if _, serr := os.Stat(abs); serr != nil {
				rel, _ := filepath.Rel(root, path)
				broken = append(broken, brokenLink{file: rel, target: link})
			}
		}
		return nil
	})
	return broken, err
}

// linkRe 粗匹配 [text](target) 与 <target>；再用 isLocalLink/cleanTarget 精筛。
//
// 为避免把行内代码（`<Service>`、`func`](...) 等）误判为链接，先剥离反引号包裹段。
func extractLinks(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var links []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := stripInlineCode(sc.Text())
		links = append(links, parseInline(line)...)
		links = append(links, parseAngle(line)...)
	}
	return links, sc.Err()
}

// stripInlineCode 把反引号包裹的行内代码段替换为等长空格，保留列位置，使后续
// 链接解析不再误抓代码示例（如 `<Service>`、`[x](y)` 出现在代码里）。
func stripInlineCode(line string) string {
	out := []byte(line)
	inCode := false
	for i := 0; i < len(out); i++ {
		if out[i] == '`' {
			inCode = !inCode
			out[i] = ' '
			continue
		}
		if inCode {
			out[i] = ' '
		}
	}
	return string(out)
}

// parseInline 提取 [text](target) 中的 target。
func parseInline(line string) []string {
	var out []string
	for {
		i := strings.Index(line, "](")
		if i < 0 {
			break
		}
		// 向前找未转义的 '['。
		open := -1
		for j := i - 1; j >= 0; j-- {
			if line[j] == '[' {
				open = j
				break
			}
		}
		start := i + 2
		end := strings.IndexByte(line[start:], ')')
		if end < 0 {
			break
		}
		target := line[start : start+end]
		if open >= 0 {
			out = append(out, target)
		}
		line = line[start+end+1:]
	}
	return out
}

// parseAngle 提取 <target> 形式的自动链接（仅本地路径）。
//
// 仅当 target 含路径分隔符或扩展名时才视为链接，避免把文档里未加反引号的
// 代码占位符（如 <Service>、<T>）误判为自动链接。
func parseAngle(line string) []string {
	var out []string
	for {
		i := strings.IndexByte(line, '<')
		if i < 0 {
			break
		}
		j := strings.IndexByte(line[i:], '>')
		if j < 0 {
			break
		}
		target := line[i+1 : i+j]
		if isLocalLink(target) && looksLikePath(target) {
			out = append(out, target)
		}
		line = line[i+j+1:]
	}
	return out
}

// looksLikePath 判定 target 是否「看起来像路径」：含路径分隔符、扩展名点，
// 或以已知 scheme 开头。纯单词（Service、T）不算。
func looksLikePath(target string) bool {
	t := cleanTarget(target)
	if strings.ContainsAny(t, "/\\") {
		return true
	}
	if strings.Contains(t, ".") {
		return true
	}
	lower := strings.ToLower(t)
	for _, p := range []string{"http://", "https://", "mailto:", "ftp://", "tel:"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// isLocalLink 判定是否为需要检查的本地链接：
//   - 排除 http(s)://、mailto:、ftp: 等外部 scheme；
//   - 排除纯锚点 #frag；
//   - 排除空的。
func isLocalLink(target string) bool {
	t := strings.TrimSpace(target)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	if i := strings.Index(t, "#"); i > 0 {
		t = t[:i] // 剥离锚点后检查路径部分
	}
	if t == "" {
		return false // 仅形如 path#frag 但 path 为空（相对锚点）
	}
	lower := strings.ToLower(t)
	for _, p := range []string{"http://", "https://", "mailto:", "ftp://", "tel:"} {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	return true
}

// cleanTarget 剥离锚点与查询串，返回纯文件路径部分。
func cleanTarget(target string) string {
	t := strings.TrimSpace(target)
	if i := strings.IndexAny(t, "#?"); i >= 0 {
		t = t[:i]
	}
	return t
}
