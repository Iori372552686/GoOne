package domain

const (
	// 类类型
	TypeOfConfig = 4
	TypeOfStruct = 3
	TypeOfEnum   = 2
	TypeOfBase   = 1

	// 值类型
	ValueOfBase  = 1
	ValueOfList  = 2
	ValueOfMap   = 3
	ValueOfGroup = 4
)

var (
	Module       = ""            // 项目目录
	ProtoPkgName = "g1.protocol" // proto包名
	PkgName      = ""            // 包名
	XlsxPath     = ""            // 解析文件路径
	ProtoPath    = ""            // proto文件路径
	PbPath       = ""            // proto生成路径
	CodePath     = ""            // 代码生成路径
	JsonPath     = ""            // 数据文件路径
	BytesPath    = ""            // 数据文件路径
	TextPath     = ""            // 数据文件路径
)
