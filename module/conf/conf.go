// Package conf 是 GoOne 的统一配置入口：多格式 decoder（yaml/toml/json）+
// 点分 key 访问 + 不可变快照 + 预留热更 Watch。
//
// 三家共识：统一用 Get(key, def...).Value 形态，放弃 GetString/GetInt 全家桶；
// 启动一次性加载到不可变快照，业务层用点分 key 取值，不再依赖包级全局变量。
//
// 读取对配置文件格式无关：所有 decoder 都输出 map[string]any，Get/Unmarshal 按点分
// 路径下钻。Load 按文件扩展名自动选 decoder，默认与现状一致走 yaml。
//
// 不可变模型：Load 后内部 atomic.Pointer 持有解析结果，发布后绝不原地修改；
// 读者无锁并发读。未来热更只需换指针（Reload），无需锁。
package conf

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultPath 是未通过命令行 -svr_conf 指定时的默认配置文件路径。
// 历史默认值 "../commconf/server_conf.yaml" 已失效（该目录不存在），改为实际存在的
// 模板路径；生产环境一律通过 -svr_conf 显式指定。
const DefaultPath = "./etc/config/server_conf.yaml"

// Path 是当前生效的配置文件路径。init() 通过 flag.StringVar 注册 -svr_conf，
// 各服务 cmd/<svc>/main.go 调用 flag.Parse 后 Path 即被命令行覆盖；未传则用
// DefaultPath。
var Path = DefaultPath

func init() {
	flag.StringVar(&Path, "svr_conf", DefaultPath, "app config file (yaml/toml/json)")
}

// ErrNotLoaded 在 Load 之前调用 Get/Has/Unmarshal 时返回（或 Get 返回默认值）。
var ErrNotLoaded = errors.New("conf: 尚未 Load，配置不可读")

// ErrAlreadyLoaded 在已成功 Load 后再次调用 Load 时返回（启动不可变模型）。
var ErrAlreadyLoaded = errors.New("conf: 已加载；启动不可变模型不支持重复 Load")

// ErrUnsupportedFormat 在配置文件扩展名未注册 decoder 时返回。
var ErrUnsupportedFormat = errors.New("conf: 不支持的配置文件格式（仅 .yaml/.yml/.toml/.json）")

// ErrWatchNotSupported 标识当前版本未实现热更 Watch。签名已定型，未来接 fsnotify
// 文件监听（配合白名单 Merger 做安全局部热更）时再启用。
var ErrWatchNotSupported = errors.New("conf: 当前版本不支持 Watch（启动不可变模型）")

// WatchCallback 是 key 变更回调签名。未来实现热更时，当 key 指向的值变化时被调用。
type WatchCallback func(key string, v Value)

// snapshot 持有当前已发布的不可变配置。零值时 current 为 nil，Get 返回默认值。
var snapshot atomic.Pointer[document]

// loadMu 串行化 Load：并发 Load 只允许第一个成功发布。
var loadMu sync.Mutex

// document 是一次解析的完整结果。data 是 decoder 产出的 map[string]any 根，
// source 记录来源（文件路径或 "<bytes>"）便于诊断。
type document struct {
	data   map[string]any
	source string
}

// Load 从 path 读取配置并发布为不可变快照。按扩展名自动选 decoder。
//
// 首次成功 Load 后再次调用返回 ErrAlreadyLoaded（启动不可变）。失败时不发布任何
// 快照。Load 前调用 Get/Has/Unmarshal 返回零值/默认值/ErrNotLoaded。
func Load(p string) error {
	if p == "" {
		return fmt.Errorf("conf: 配置路径为空")
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("conf: 读取配置文件 %q 失败: %w", p, err)
	}
	if err := loadLocked(content, extOf(p), p); err != nil {
		return err
	}
	return nil
}

// LoadBytes 从内存字节加载并发布快照，ext 指定格式（".yaml"/".yml"/".toml"/".json"）。
// 主要用于测试与嵌入式场景。同样受 Load 幂等约束。
func LoadBytes(content []byte, ext string) error {
	return loadLocked(content, ext, "<bytes>")
}

// loadLocked 是 Load/LoadBytes 的共享实现。串行化保证并发 Load 只发布一次。
func loadLocked(content []byte, ext, source string) error {
	loadMu.Lock()
	defer loadMu.Unlock()
	if snapshot.Load() != nil {
		return ErrAlreadyLoaded
	}
	decode, ok := decoders[strings.ToLower(ext)]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, ext)
	}
	data, err := decode(content)
	if err != nil {
		return fmt.Errorf("conf: 解析配置 %q 失败: %w", source, err)
	}
	snapshot.Store(&document{data: data, source: source})
	return nil
}

// extOf 返回小写扩展名（含点），如 ".yaml"。无扩展名返回空串（后续报 ErrUnsupportedFormat）。
func extOf(p string) string { return strings.ToLower(filepath.Ext(p)) }

// loaded 报告是否已完成 Load。
func loaded() bool { return snapshot.Load() != nil }

// Current 返回当前快照的只读视图（整个文档根）。未 Load 时返回 nil。
// 返回的 map 由快照独占，调用方不得修改（不可变契约）。
func Current() map[string]any {
	if d := snapshot.Load(); d != nil {
		return d.data
	}
	return nil
}

// source 返回当前快照来源描述，便于错误诊断。未 Load 时返回 "<not loaded>"。
func source() string {
	if d := snapshot.Load(); d != nil {
		return d.source
	}
	return "<not loaded>"
}

// reset 清空当前快照，仅用于测试。生产配置启动不可变，无重置入口。
func reset() {
	loadMu.Lock()
	defer loadMu.Unlock()
	snapshot.Store(nil)
}

// ResetForTest 清空当前快照，仅用于测试。跨包测试（如业务层单测）需要重置 conf
// 状态时使用；生产代码不得调用。
func ResetForTest() { reset() }

// Get 按点分 key 读取配置，返回类型化 Value。key 支持数组下标，例如
// "base_cfg.dependencies.db_instances.0.ip"。
//
// 未 Load 或 key 不存在时：若提供 def 则用 def[0] 构造 Value，否则返回不存在
// 的 Value（Exists()==false，类型化方法返回零值）。
func Get(key string, def ...any) Value {
	if d := snapshot.Load(); d != nil {
		if v, ok := lookup(d.data, key); ok {
			return newValue(v, true)
		}
	}
	if len(def) > 0 {
		return newValue(def[0], true)
	}
	return newValue(nil, false)
}

// Has 报告 key 是否存在（且已 Load）。
func Has(key string) bool {
	if d := snapshot.Load(); d == nil {
		return false
	} else if _, ok := lookup(d.data, key); ok {
		return true
	}
	return false
}

// Unmarshal 把 key 指向的子树反序列化到 out（out 必须是指针）。
// 内部走 yaml 中转：子树先 marshal 成 yaml 字节再 unmarshal 进 out，使 out 复用
// 现有 yaml tag（与 gconf struct 既有约定一致），无需为每个格式单独打 tag。
func Unmarshal(key string, out any) error {
	if !loaded() {
		return ErrNotLoaded
	}
	v, ok := lookup(snapshot.Load().data, key)
	if !ok {
		return fmt.Errorf("conf: key %q 不存在", key)
	}
	return reencode(v, out)
}

// Watch 注册 key 变更回调。当前版本返回 ErrWatchNotSupported（启动不可变模型）。
// 签名定型后，未来接 fsnotify 文件监听 + 白名单 Merger 做安全局部热更。
// 返回的 cancel 在未来实现时用于注销回调；当前恒为 no-op。
func Watch(key string, cb WatchCallback) (cancel func(), err error) {
	_ = key
	_ = cb
	return func() {}, ErrWatchNotSupported
}

// lookup 在根 map 按点分 key 下钻。遇到 []any 用整数下标取元素。
// 不支持通配符。找不到返回 (nil, false)。
func lookup(root map[string]any, key string) (any, bool) {
	if key == "" {
		return root, true
	}
	var cur any = root
	for _, seg := range strings.Split(key, ".") {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// atoi 是 strconv.Atoi 的简化封装；非整数返回错误。
func atoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
