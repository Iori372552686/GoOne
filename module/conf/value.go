package conf

import (
	"fmt"
	"strconv"
	"time"
)

// Value 是 Get 返回的类型化访问对象。统一对象形态（放弃 GetString/GetInt 全家桶）
// 是 due/kratos/goframe 三家的共识：API 面积最小、扩展性最好。
type Value interface {
	// Exists 报告 key 是否存在且已 Load。默认值构造的 Value 也返回 true。
	Exists() bool
	// Raw 返回底层原始值（未经类型转换）。
	Raw() any
	// 类型化访问；不存在或转换失败时返回该类型的零值。
	String() string
	Int() int
	Int64() int64
	Bool() bool
	Float64() float64
	// Duration 把字符串/数值解析为 time.Duration。字符串支持 "5s"/"100ms"；
	// 数值按纳秒解释（与 gf 一致）。
	Duration() time.Duration
	// 集合访问。单个标量会被包成单元素切片以容错。
	Strings() []string
	Ints() []int
	// StringsMap 读取 map[string]string（如 tracing.headers）。
	StringsMap() map[string]string
	// Scan 等价于 Unmarshal(key, out)：把子树反序列化进 out（指针）。
	Scan(out any) error
}

// valueImpl 是 Value 的唯一实现。exists 区分"key 不存在"与"值为 nil"。
type valueImpl struct {
	raw    any
	exists bool
}

// newValue 构造 Value。exists=false 时所有类型化方法返回零值。
func newValue(raw any, exists bool) Value {
	return &valueImpl{raw: raw, exists: exists}
}

func (v *valueImpl) Exists() bool { return v.exists }
func (v *valueImpl) Raw() any    { return v.raw }

func (v *valueImpl) String() string {
	if v.raw == nil {
		return ""
	}
	switch t := v.raw.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func (v *valueImpl) Int() int { return int(v.Int64()) }

func (v *valueImpl) Int64() int64 {
	if v.raw == nil {
		return 0
	}
	switch t := v.raw.(type) {
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint:
		return int64(t)
	case uint8:
		return int64(t)
	case uint16:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

func (v *valueImpl) Bool() bool {
	if v.raw == nil {
		return false
	}
	switch t := v.raw.(type) {
	case bool:
		return t
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v.Int64() != 0
	case float32, float64:
		return v.Float64() != 0
	case string:
		b, _ := strconv.ParseBool(t)
		return b
	default:
		return false
	}
}

func (v *valueImpl) Float64() float64 {
	if v.raw == nil {
		return 0
	}
	switch t := v.raw.(type) {
	case float32:
		return float64(t)
	case float64:
		return t
	case int:
		return float64(t)
	case int8:
		return float64(t)
	case int16:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case uint:
		return float64(t)
	case uint8:
		return float64(t)
	case uint16:
		return float64(t)
	case uint32:
		return float64(t)
	case uint64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func (v *valueImpl) Duration() time.Duration {
	if v.raw == nil {
		return 0
	}
	switch t := v.raw.(type) {
	case string:
		d, err := time.ParseDuration(t)
		if err != nil {
			return 0
		}
		return d
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// 数值按纳秒解释（与 gf 一致），便于配置 "5_000_000_000" 这类无单位值。
		return time.Duration(v.Int64())
	case float32, float64:
		return time.Duration(v.Float64())
	default:
		return 0
	}
}

func (v *valueImpl) Strings() []string {
	switch t := v.raw.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, newValue(e, true).String())
		}
		return out
	case []string:
		return append([]string(nil), t...)
	case string:
		return []string{t}
	case nil:
		return nil
	default:
		return []string{v.String()}
	}
}

func (v *valueImpl) Ints() []int {
	switch t := v.raw.(type) {
	case []any:
		out := make([]int, 0, len(t))
		for _, e := range t {
			out = append(out, newValue(e, true).Int())
		}
		return out
	case []int:
		return append([]int(nil), t...)
	case nil:
		return nil
	default:
		return []int{v.Int()}
	}
}

func (v *valueImpl) Scan(out any) error {
	if !v.exists {
		return fmt.Errorf("conf: value 不存在，无法 Scan")
	}
	return reencode(v.raw, out)
}

// StringsMap 把 map[string]any 类型的值转换成 map[string]string，值的每个元素
// 走 String() 转换。非 map 类型返回 nil。
func (v *valueImpl) StringsMap() map[string]string {
	switch t := v.raw.(type) {
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, val := range t {
			out[k] = newValue(val, true).String()
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, val := range t {
			out[k] = val
		}
		return out
	default:
		return nil
	}
}
