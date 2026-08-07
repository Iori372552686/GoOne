package domain

const (
	// 类类型：描述元素「是什么」（标量/枚举/结构体/配置表）
	TypeOfConfig = 4
	TypeOfStruct = 3
	TypeOfEnum   = 2
	TypeOfBase   = 1

	// 字段值容器类别：描述字段「以什么形态承载」
	// 与 TypeOf 正交，字段专用；ContainerArray 的维度由 Type.ArrayDepth 表达
	ContainerSingle = 0 // 单值
	ContainerArray  = 1 // 数组（[]int32 / [][]int64 / []Reward 等）
	ContainerMap    = 2 // map（map[K]V，K/V 由 Type.KeyType / Type.Name 表达）

	// 索引语义：仅 Index 用，描述配置表「以何种方式组织/查询」
	// （List=顺序遍历，Map=主键唯一查找，Group=一对多分组）
	// 注意：与字段值容器是两个独立维度，勿混用
	ValueOfBase  = 1
	ValueOfList  = 2
	ValueOfMap   = 3
	ValueOfGroup = 4

	MaxArrayDepth = 3
)

var (
	Version      = "1.0.7"       // 当前版本号
	Module       = ""            // 项目目录
	ConfMode     = ""            // 配置gen模式（all：全部  client：客户端  server：服务器）；xlsx第四行支持 key/KEY 标记主键索引
	ProtoPkgName = "g1.protocol" // proto包名
	PkgName      = ""            // 包名
	XlsxPath     = ""            // 解析文件路径
	ProtoPath    = ""            // proto文件路径
	PbPath       = ""            // proto生成路径
	CodePath     = ""            // 代码生成路径
	CppPath      = ""            // C++代码生成路径
	NodeJsPath   = ""            // Node.js/TypeScript代码生成路径
	JsonPath     = ""            // 数据文件路径
	BytesPath    = ""            // 数据文件路径
	TextPath     = ""            // 数据文件路径
	LuaPath      = ""            // lua数据文件路径
	TsPath       = ""            // ts数据文件路径
)
