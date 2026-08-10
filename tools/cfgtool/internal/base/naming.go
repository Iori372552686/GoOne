package base

import (
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
)

// 命名约定（功能名@表名 体系）：
//
//   xlsx 文件名: <中文名>@<功能名>.xlsx   例如 掉落系统@drop.xlsx  -> feature=drop
//   sheet    名: <中文名>@<表名>          例如 掉落组表@group      -> table=group
//
// 无 @ 时退化为原行为（功能名=文件基名 / 表名=sheet 名本身），保持向后兼容。
//
// 派生命名（以 drop+group 为例）：
//   Config 类型名 : DropGroupConfig        (PascalCase(feature)+PascalCase(table)+"Config")
//   proto 文件名  : drop.proto             (= feature，决定 proto 分桶)
//   Go 子目录/pkg : drop                   (snake_case(feature))
//   Go 文件名     : gdata_drop_group.go    ("gdata_"+feature+"_"+table)
//   数据文件名    : DropGroupConfig.conf   (= 类型名)

const (
	// nameSeparator 是「中文名@标识名」体系中的分隔符。
	nameSeparator = "@"
	// goFilePrefix 是生成 Go 配置文件的前缀。
	goFilePrefix = "gdata_"
	// configTypeSuffix 是 Config 类型名的固定后缀。
	configTypeSuffix = "Config"
)

// ParseFileFeature 从 xlsx 文件名解析功能名。
//   "掉落系统@drop.xlsx" -> ("drop", "掉落系统")
//   "DropItem.xlsx"      -> ("DropItem", "")   （无 @ 退化：功能名=文件基名）
//
// 第二个返回值是中文名（仅用于日志/注释），无 @ 时为空。
func ParseFileFeature(fileName string) (feature, chinese string) {
	base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	if idx := strings.Index(base, nameSeparator); idx > 0 {
		return base[idx+1:], base[:idx]
	}
	return base, ""
}

// ParseSheetTable 从 sheet 名解析表名。
//   "掉落组表@group" -> ("group", "掉落组表")
//   "DropItem"       -> ("DropItem", "")   （无 @ 退化：表名=sheet 名本身）
func ParseSheetTable(sheetName string) (table, chinese string) {
	if idx := strings.Index(sheetName, nameSeparator); idx > 0 {
		return sheetName[idx+1:], sheetName[:idx]
	}
	return sheetName, ""
}

// ConfigTypeName 生成 Config 类型名（proto message / Go 类型 / register key）。
//   ("drop", "group")    -> "DropGroupConfig"     （正常：功能名+表名+Config）
//   ("decompose", "decompose") -> "DecomposeConfig" （去重：feature/table 同名时不重复拼接）
//   ("item", "item")     -> "ItemConfig"
// 规则：当 PascalCase(feature) 与 PascalCase(table) 相等时，类型名 = Pascal + "Config"，
// 避免出现 DecomposeDecomposeConfig 这种冗余。正常情况下两者不同，直接拼接。
// 注：strcase.ToCamel 即 PascalCase（首字母大写驼峰）。
func ConfigTypeName(feature, table string) string {
	pf := strcase.ToCamel(feature)
	pt := strcase.ToCamel(table)
	if pf == pt {
		return pf + configTypeSuffix
	}
	return pf + pt + configTypeSuffix
}

// ProtoFileName 生成 proto 文件基名（不含 .proto 后缀）。
//   "drop" -> "drop"
// 保持为独立函数以便未来调整（如加前缀/命名空间）。
func ProtoFileName(feature string) string {
	return feature
}

// GoPkgName 生成 Go package 名 / 子目录名。
//   "drop"      -> "drop"
//   "DropItem"  -> "drop_item"
func GoPkgName(feature string) string {
	return strcase.ToSnake(feature)
}

// GoFileName 生成 Go 配置文件名。
//   ("drop", "group")    -> "gdata_drop_group.go"
//   ("dropitem", "item") -> "gdata_dropitem_item.go"
func GoFileName(feature, table string) string {
	return goFilePrefix + feature + "_" + table + ".go"
}
