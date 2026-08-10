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
	// IsExternal 表示该字段类型引用自外部 proto（pb.XXX 语法）。
	// 用于 parseReference 收集外部 import、data_gen 走反射驱动赋值。
	IsExternal bool
}

type Struct struct {
	Name      string              // 结构体名称
	Fields    map[string]*Field   // 字段类型
	FieldList []*Field            // 字段类型
	Converts  map[string][]*Field // 转换表
	Sheet     string
	FileName  string // 文件名（= 功能名，决定 proto 分桶）
	Feature   string // 功能名（从 xlsx 文件名 @ 前解析；= FileName，保留以便生成阶段直读）
	Table     string // 表名（从 sheet 名 @ 前解析）；struct 通常为空
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
	FileName  string // 文件名（= 功能名，决定 proto 分桶）
	Feature   string // 功能名（从 xlsx 文件名 @ 前解析）；决定 Go 子目录/package
	Table     string // 表名（从 sheet 名 @ 前解析）；决定 Go 文件名 gdata_<feature>_<table>.go
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
	FileName  string // 文件名（= 功能名，决定 proto 分桶）
	Feature   string // 功能名（从 xlsx 文件名 @ 前解析）
}

type Table struct {
	TypeOf   int
	Sheet    string
	Type     string
	FileName string // 文件名（= 功能名，决定 proto 分桶）
	Feature  string // 功能名（从 xlsx 文件名 @ 前解析；= FileName，保留以便生成阶段直读）
	Table    string // 表名（从 sheet 名 @ 前解析）
	Rules    []string
	Rows     [][]string
	LuaRules string // lua规则
	MapRules string // map 索引规则
}
