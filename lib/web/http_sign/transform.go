package http_sign

import (
	"net/url"
	"sort"
	"strings"
)

// UriParam2Map 将 URL 原始查询串（"a=1&b=2"）解析为 map。
//
// 值中包含 '=' 会被完整保留（使用 SplitN 限制切分次数 2），因此形如
// "url=http://x?a=1" 的查询会把 "url" 映射为 "http://x?a=1"。
// 重复 key 取最后出现的值，符合常见的查询串语义。
func UriParam2Map(rawQuery string) map[string]string {
	m := make(map[string]string)
	if rawQuery == "" {
		return m
	}
	for _, pair := range strings.Split(rawQuery, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		m[k] = v
	}
	return m
}

// MapParam2Uri 将 params 序列化为 "k=v&k=v" 查询串，不排序，
// 可选地对值做 URL 编码。它是不带字段过滤的 Map2uri 外部封装。
func MapParam2Uri(params map[string]string, encode bool) string {
	return Map2uri(params, "", false, encode)
}

// Map2uri 将 params 序列化为 "k=v&k=v" 查询串。
//
// 参数：
//   - filter   ：需排除的字段名（例如签名字段本身）。
//   - sortKeys ：为 true 时按 key 字典序升序输出，保证结果稳定、可用于签名。
//   - encode   ：为 true 时对值做 url.QueryEscape。
//
// 注意：为向后兼容，空值会被跳过——这意味着 "foo=" 不会影响签名；
// 改动该行为会使所有已签发的签名失效，因此明确不做修改。
func Map2uri(params map[string]string, filter string, sortKeys, encode bool) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		if k != filter {
			keys = append(keys, k)
		}
	}
	if sortKeys {
		sort.Strings(keys)
	}

	var b strings.Builder
	b.Grow(len(params) * 16) // 预估容量，避免大多数扩容
	first := true
	for _, k := range keys {
		v := params[k]
		if v == "" {
			continue
		}
		if encode {
			v = url.QueryEscape(v)
		}
		if !first {
			b.WriteByte('&')
		}
		first = false
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
	}
	return b.String()
}
