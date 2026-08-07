package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/lib/contrib/config/factory"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
)

// UploadItem 描述一种要上传的产物：源目录、文件后缀、发布时声明的格式。
type UploadItem struct {
	Dir    string
	Suffix string // 含点，如 ".json"
	Format string // 发布格式提示，如 "json"
}

// ParseUploadType 把 -uptype（逗号分隔）解析为按声明顺序去重的上传项列表。
// 未知 token 记 warn 并跳过；同一 token 重复出现只保留首次。
// 返回 nil 表示没有任何有效项（调用方据此决定是否仍构造 Publisher）。
func ParseUploadType(raw string) []UploadItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// kind → (源目录, 后缀, 格式)
	table := map[string]UploadItem{
		"json":  {Dir: domain.JsonPath, Suffix: ".json", Format: "json"},
		"conf":  {Dir: domain.TextPath, Suffix: ".conf", Format: "text"},
		"bytes": {Dir: domain.BytesPath, Suffix: ".bytes", Format: "bytes"},
		"lua":   {Dir: domain.LuaPath, Suffix: ".lua", Format: "text"},
	}

	seen := map[string]struct{}{}
	out := make([]UploadItem, 0, 4)
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(strings.ToLower(tok))
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		item, ok := table[tok]
		if !ok {
			logx.Warnf("未知 uptype=%q 已忽略（支持：json/conf/bytes/lua）\n", tok)
			continue
		}
		out = append(out, item)
	}
	return out
}

// UploadData 按 -upload（URL）与 -uptype（格式集合）把已生成的数据产物上传到配置中心。
//
// 设计要点：
//   - -upload 为空 → 直接返回（不上传）。
//   - 通过 lib/contrib/config/factory.NewPublisherFromURL 解析 URL 并构造 Publisher，
//     与 common/gamedata 读路径使用同一套地址/鉴权解析。
//   - GenData() 末尾已 manager.Clear()，故这里直接扫各产物目录而非读内存。
//   - 每种产物（json/conf/bytes/lua）各自 glob 对应后缀，逐文件 Publish；
//     dataID = filepath.Base(file)（如 "ItemConfig.json"）。
func UploadData() error {
	if strings.TrimSpace(domain.UploadURL) == "" {
		return nil
	}
	items := ParseUploadType(domain.UploadType)
	if len(items) == 0 {
		logx.Warnf("-upload 已指定但 -uptype 为空或无有效项，跳过上传\n")
		return nil
	}

	pub, cfg, err := factory.NewPublisherFromURL(domain.UploadURL)
	if err != nil {
		return errs.Wrap(err, "", "", "", 0, "上传错误", "构造配置中心 Publisher 失败: %s", domain.UploadURL)
	}
	defer func() { _ = pub.Close() }()
	logx.Infof("已连接配置中心后端=%s，开始上传\n", cfg.Backend)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	total := 0
	for _, it := range items {
		if strings.TrimSpace(it.Dir) == "" {
			logx.Warnf("uptype=%s 对应的源目录为空（未指定 -json/-text/-bytes/-lua 之一），跳过\n", strings.TrimPrefix(it.Suffix, "."))
			continue
		}
		pattern := ".*" + regexpQuote(it.Suffix) + "$"
		files, gerr := base.Glob(it.Dir, pattern, false)
		if gerr != nil {
			return errs.Wrap(gerr, "", "", "", 0, "上传错误", "扫描目录失败: %s", it.Dir)
		}
		if len(files) == 0 {
			logx.Warnf("目录 %s 无 %s 文件，跳过\n", it.Dir, it.Suffix)
			continue
		}
		for _, f := range files {
			content, ferr := os.ReadFile(f)
			if ferr != nil {
				return errs.Wrap(ferr, filepath.Base(f), "", "", 0, "上传错误", "读取文件失败: %s", f)
			}
			dataID := filepath.Base(f)
			if perr := pub.Publish(ctx, dataID, content, it.Format); perr != nil {
				return errs.Wrap(perr, dataID, "", "", 0, "上传错误", "发布失败: %s (format=%s)", dataID, it.Format)
			}
			total++
			logx.Infof("已上传 %s (%s, %d bytes)\n", dataID, it.Format, len(content))
		}
	}
	logx.Successf("配置数据上传完成，共 %d 项", total)
	return nil
}

// regexpQuote 对后缀中的元字符做转义（".json" 里的 "." 需转义为 "\\."）。
func regexpQuote(s string) string {
	// 只需处理后缀里可能出现的小集合，简单逐字符转义非字母数字。
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}
