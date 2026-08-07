package manager

import (
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

var (
	tableMgr = make(map[string]*base.Table)
	groupMgr = make(map[int][]*base.Table)
)

func AddTable(file, sheet string, typeOf int, t string, rows [][]string, rules []string) {
	key := file + ":" + sheet
	val := &base.Table{
		Type:     t,
		TypeOf:   typeOf,
		Sheet:    sheet,
		FileName: file,
		Rules:    rules,
		Rows:     rows,
	}

	for i, rule := range rules {
		//根据：分割，判断前面的字符串，是否等于"lua"
		pos := strings.Index(rule, ":")
		if pos > 0 && strings.ToLower(rule[:pos]) == "lua" {
			val.LuaRules = rules[i]
		}
	}

	tableMgr[key] = val
	groupMgr[val.TypeOf] = append(groupMgr[val.TypeOf], val)
}

func GetTable(file, sheet string) *base.Table {
	return tableMgr[file+":"+sheet]
}

func GetTableList(typeOf int) []*base.Table {
	return groupMgr[typeOf]
}

func GetTypeOf(name string) int {
	name = GetConvType(name)
	if _, ok := enumMgr[name]; ok {
		return domain.TypeOfEnum
	}
	if _, ok := structMgr[name]; ok {
		return domain.TypeOfStruct
	}
	return domain.TypeOfBase
}

func SplitArrayType(name string) (string, int) {
	depth := 0
	for strings.HasPrefix(name, "[]") {
		depth++
		name = strings.TrimPrefix(name, "[]")
	}
	return name, depth
}

func GetValueOf(name string) int {
	_, depth := SplitArrayType(name)
	if depth > 0 {
		return domain.ValueOfList
	}
	return domain.ValueOfBase
}

// SplitMapType 解析 map[K]V 形式的字段类型。
// 命中返回 (K 原始名, V 原始名, true)；否则返回 ok=false。
// 仅识别最外层的 map（不递归，嵌套 map 本期不支持）。
// 注意：V 是首个 ']' 之后的全部内容（V 不被 ']' 包裹），故不能用 HasSuffix("]")。
func SplitMapType(name string) (key, val string, ok bool) {
	const prefix = "map["
	if !strings.HasPrefix(name, prefix) {
		return "", "", false
	}
	rest := name[len(prefix):]
	idx := strings.IndexByte(rest, ']')
	if idx < 0 {
		return "", "", false
	}
	return rest[:idx], rest[idx+1:], true
}

// GetContainerOf 根据类型名前缀判定字段值容器类别。
// map[K]V -> ContainerMap；[]... 前缀 -> ContainerArray；其余 -> ContainerSingle。
func GetContainerOf(name string) int {
	if _, _, isMap := SplitMapType(name); isMap {
		return domain.ContainerMap
	}
	if _, depth := SplitArrayType(name); depth > 0 {
		return domain.ContainerArray
	}
	return domain.ContainerSingle
}
