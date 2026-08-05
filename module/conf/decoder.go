package conf

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/util/encoding"
	"github.com/Iori372552686/GoOne/lib/util/encoding/json"
	"github.com/Iori372552686/GoOne/lib/util/encoding/toml"
	"github.com/Iori372552686/GoOne/lib/util/encoding/yaml"
)

// DecodeFunc 把配置文件字节解析为 map[string]any 根。所有格式统一产出该类型，
// 使后续 Get/Unmarshal 完全格式无关。
type DecodeFunc func(content []byte) (map[string]any, error)

// decoders 按小写扩展名（含点）注册。Load 按文件扩展名查表分发；未注册的扩展名
// 报 ErrUnsupportedFormat（不再静默回退 JSON，修复历史坏味道）。
//
// 复用 lib/util/encoding 的 Codec 实现（yaml/toml/json 子包），不在此重新
// import yaml.v3/BurntSushi/toml/encoding/json，保持项目编码能力单一来源。
var decoders = map[string]DecodeFunc{
	".yaml": decodeByCodec(yaml.Name),
	".yml":  decodeByCodec(yaml.Name),
	".toml": decodeByCodec(toml.Name),
	".json": decodeByCodec(json.Name),
}

// decodeByCodec 用 encoding 注册表里的 Codec 解析为 map[string]any。
// 各 Codec.Unmarshal 到 *map[string]any 时：
//   - yaml/json：嵌套对象→map[string]any，数组→[]any，与 lookup 直接兼容；
//   - toml：表→map[string]any，数组→[]map[string]any（非 []any），
//     normalizeContainer 把它统一成 []any 以匹配 lookup 的下钻假设。
func decodeByCodec(name string) DecodeFunc {
	return func(content []byte) (map[string]any, error) {
		c := encoding.GetCodec(name)
		if c == nil {
			return nil, fmt.Errorf("conf: codec %q 未注册", name)
		}
		var out map[string]any
		if err := c.Unmarshal(content, &out); err != nil {
			return nil, err
		}
		normalized := normalizeContainer(out)
		m, _ := normalized.(map[string]any)
		if m == nil {
			return map[string]any{}, nil
		}
		return m, nil
	}
}

// normalizeContainer 递归把任意解析器产出的容器统一成 map[string]any / []any。
// Go 中 map[string]any 与 map[string]interface{} 是同一类型，无需分别处理。
// TOML 的 []map[string]any 需要转成 []any 以匹配 lookup 的下钻假设；
// 对 yaml/json 的输出调用本函数是幂等的（类型已对齐，各 case 不命中）。
func normalizeContainer(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalizeContainer(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = normalizeContainer(val)
		}
		return t
	case []map[string]any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeContainer(val)
		}
		return out
	default:
		return v
	}
}

// reencode 把 lookup 取出的子树中转反序列化到 out。子树先 marshal 成 yaml 字节
// 再 unmarshal 进 out，使 out 复用既有 yaml tag（与 gconf struct 的 yaml 约定一致），
// 调用方无需为 toml/json 单独打 tag。这也是 kratos Scan 的等价做法。
//
// 复用 encoding/yaml Codec，不直接 import yaml.v3。
func reencode(v any, out any) error {
	c := encoding.GetCodec(yaml.Name)
	if c == nil {
		return fmt.Errorf("conf: yaml codec 未注册")
	}
	b, err := c.Marshal(v)
	if err != nil {
		return fmt.Errorf("conf: 中转 marshal 失败: %w", err)
	}
	if err := c.Unmarshal(b, out); err != nil {
		return fmt.Errorf("conf: 中转 unmarshal 失败: %w", err)
	}
	return nil
}
