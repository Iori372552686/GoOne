package base

type Type struct {
	Name    string
	TypeOf  int
	ValueOf int
	// ArrayDepth 表示数组维度：0=标量，1=一维，2=二维，3=三维。
	// 仅 ContainerArray 有效。
	ArrayDepth int
	// Container 描述字段值容器类别（domain.ContainerSingle/Array/Map）。
	// 与 TypeOf 正交：TypeOf 描述元素种类，Container 描述承载形态。
	Container int
	// KeyType 仅在 Container==ContainerMap 时填充，描述 map 的 K。
	// map 的 V 复用本 Type 的 Name/TypeOf 表达。
	KeyType *Type
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
	LuaRules string // lua规则
	MapRules string // map 索引规则
}
