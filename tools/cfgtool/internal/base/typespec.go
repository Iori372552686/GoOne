package base

type Type struct {
	Name    string
	TypeOf  int
	ValueOf int
}

type Convert struct {
	Name     string                   // 装换类型
	ConvFunc func(string) interface{} // 装换函数
}

type Field struct {
	Type     *Type
	Name     string
	Desc     string
	Position int
	ConvFunc func(string) interface{} // 装换函数
}

type Struct struct {
	Name      string              // 结构体名称
	Fields    map[string]*Field   // 字段类型
	FieldList []*Field            // 字段类型
	Converts  map[string][]*Field // 转换表
	Sheet     string
	FileName  string // 文件名
}

type Index struct {
	Type *Type    // 成员变量类型
	Name string   // 成员变量
	List []*Field // 类型字段
}

type Config struct {
	Name      string
	Fields    map[string]*Field
	FieldList []*Field
	Indexs    map[int][]*Index
	IndexList []*Index
	Sheet     string
	FileName  string
}

type EValue struct {
	Name  string // 枚举值名称
	Value int32  // 枚举值
	Desc  string // 枚举值描述
}

type Enum struct {
	Name      string
	Values    map[string]*EValue
	ValueList []*EValue
	Sheet     string
	FileName  string
}

type Table struct {
	TypeOf   int
	Sheet    string
	Type     string
	FileName string
	Rules    []string
	Rows     [][]string
}
