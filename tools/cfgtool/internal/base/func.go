package base

import (
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/spf13/cast"
)

// E|道具类型-金币|PropertyType|Coin|1
func (d *Enum) AddValue(strs ...string) {
	val := &EValue{
		Name:  strs[2] + "_" + strs[3],
		Desc:  strs[1],
		Value: cast.ToInt32(strs[4]),
	}
	d.ValueList = append(d.ValueList, val)
	d.Values[val.Desc] = val
}

func (d *Struct) AddField(f *Field) {
	d.Fields[f.Name] = f
	d.FieldList = append(d.FieldList, f)
}

func (d *Config) AddField(f *Field) {
	d.Fields[f.Name] = f
	d.FieldList = append(d.FieldList, f)
}

func (d *Config) AddIndex(ind *Index) {
	d.Indexs[ind.Type.ValueOf] = append(d.Indexs[ind.Type.ValueOf], ind)
	d.IndexList = append(d.IndexList, ind)
}

// 成员函数参数
func (d *Index) Arg(split string) string {
	strs := []string{}
	for _, val := range d.List {
		strs = append(strs, val.Name+" "+val.Type.GetType())
	}
	return strings.Join(strs, split)
}

func (d *Index) Value(ref, split string) string {
	strs := []string{}
	for _, val := range d.List {
		if len(ref) > 0 {
			strs = append(strs, ref+"."+val.Name)
		} else {
			strs = append(strs, val.Name)
		}
	}
	return strings.Join(strs, split)
}

// KeyStructInit 生成复合键 Index 结构体的命名字段初始化表达式。
// ref 为字段来源引用前缀（如 "item"），为空则用裸字段名。
// 字段名约定与 index_tpl.go 的 member 模板一致：T{N}, T{N-1}, ..., T{1}
// （高位在前），与 List 顺序对应（List[0]=T{N}, List[1]=T{N-1}, ...）。
//
// 示例（2 维，ref="item"）：
//
//	T2: item.RoomStage, T1: item.CoinType
//
// 用于替代位置初始化 {a, b}，消除 go vet 的 unkeyed fields 警告。
func (d *Index) KeyStructInit(ref string) string {
	n := len(d.List)
	strs := make([]string, 0, n)
	for i, val := range d.List {
		field := val.Name
		if ref != "" {
			field = ref + "." + val.Name
		}
		// List[i] 对应 T{n-i}：List[0]=T{n}, List[1]=T{n-1}...
		strs = append(strs, fmt.Sprintf("T%d: %s", n-i, field))
	}
	return strings.Join(strs, ", ")
}

func (d *Type) arrayDepth() int {
	if d.ArrayDepth > 0 {
		return d.ArrayDepth
	}
	if d.ValueOf == domain.ValueOfList && d.Name != "" {
		return 1
	}
	return 0
}

// 获取类型字符串
func (d *Type) GetType() string {
	depth := d.arrayDepth()
	switch d.TypeOf {
	case domain.TypeOfBase:
		return strings.Repeat("[]", depth) + d.Name
	case domain.TypeOfEnum:
		name := d.Name
		if len(domain.PkgName) > 0 {
			name = domain.PkgName + "." + d.Name
		}
		return strings.Repeat("[]", depth) + name
	case domain.TypeOfStruct, domain.TypeOfConfig:
		name := "*" + d.Name
		if len(domain.PkgName) > 0 {
			name = fmt.Sprintf("*%s.%s", domain.PkgName, d.Name)
		}
		return strings.Repeat("[]", depth) + name
	}
	return ""
}

func ArrayWrapperName(fileName, elemName string, level int) string {
	if level <= 0 {
		return elemName
	}
	hash := crc32.ChecksumIEEE([]byte(fileName + "|" + elemName))
	return fmt.Sprintf("PBARR_%s_%s_%d_%08x", sanitizeProtoIdent(fileName), sanitizeProtoIdent(elemName), level, hash)
}

func sanitizeProtoIdent(s string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		isAsciiLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		switch {
		case isAsciiLetter || isDigit:
			if b.Len() == 0 && isDigit {
				b.WriteString("X_")
			}
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore && b.Len() > 0:
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "X"
	}
	return out
}

// CppArg 生成 C++ 风格参数签名，如 "int32_t Id, const std::string& Name"
func (d *Index) CppArg(split string) string {
	strs := []string{}
	for _, val := range d.List {
		strs = append(strs, val.Type.GetCppType()+" "+val.Name)
	}
	return strings.Join(strs, split)
}

// CppKeyType 返回 C++ map 键类型：单字段用原生类型，多字段用 std::string
func (d *Index) CppKeyType() string {
	if len(d.List) == 1 {
		return d.List[0].Type.GetCppType()
	}
	return "std::string"
}

// CppKeyExpr 生成取键表达式，ref 为变量引用（如 "item"）。
// 单字段：item.field_name()   多字段：拼接为 string
func (d *Index) CppKeyExpr(ref string) string {
	if len(d.List) == 0 {
		return `""`
	}
	if len(d.List) == 1 {
		f := d.List[0]
		if ref != "" {
			return ref + "." + strings.ToLower(f.Name) + "()"
		}
		return f.Name
	}
	// 多字段：用 ostringstream
	parts := []string{}
	for _, f := range d.List {
		if ref != "" {
			parts = append(parts, ref+"."+strings.ToLower(f.Name)+"()")
		} else {
			parts = append(parts, f.Name)
		}
	}
	return strings.Join(parts, ` << "_" << `)
}

// CppKeyBuild 生成构建键的完整语句（用在 Parse 或 Getter 中）
// 单字段返回直接值，多字段返回 ostringstream 表达式
func (d *Index) CppKeyBuild(ref string) string {
	if len(d.List) == 1 {
		f := d.List[0]
		if ref != "" {
			return ref + "." + strings.ToLower(f.Name) + "()"
		}
		return f.Name
	}
	// 需要在模板中用 [&]() { std::ostringstream oss; oss << ...; return oss.str(); }()
	return "" // 模板中针对多字段单独处理
}

// GetCppType proto 类型 -> C++ 类型
func (d *Type) GetCppType() string {
	switch d.Name {
	case "int32":
		return "int32_t"
	case "int64":
		return "int64_t"
	case "uint32":
		return "uint32_t"
	case "uint64":
		return "uint64_t"
	case "float":
		return "float"
	case "double":
		return "double"
	case "string":
		return "const std::string&"
	case "bool":
		return "bool"
	default:
		return "const std::string&"
	}
}

// CppKeyParam 返回 C++ 参数中的键类型（值传递版本，去掉 const &）
func (d *Type) GetCppParamType() string {
	switch d.Name {
	case "string":
		return "std::string"
	default:
		return d.GetCppType()
	}
}

func (d *Field) Convert(vals ...string) (rets []interface{}) {
	for _, val := range vals {
		rets = append(rets, d.ConvFunc(val))
	}
	return
}

type FieldList []*Field

// indexPkg 是复合键容器类型（Index2/3/4）所在的 Go 包名。
// 这些类型手写在 module/gamedata/index.go，生成代码已 import gamedata（用于 Register），
// 故直接引用 gamedata.Index{N}，不再依赖 protocol 包或单独生成的 index.gen.go。
const indexPkg = "gamedata"

func (d FieldList) GetIndexName() string {
	if len(d) == 1 {
		return d[0].Type.GetType()
	}
	strs := []string{}
	for _, val := range d {
		strs = append(strs, val.Type.GetType())
	}
	return fmt.Sprintf("%s.Index%d[%s]", indexPkg, len(d), strings.Join(strs, ","))
}
