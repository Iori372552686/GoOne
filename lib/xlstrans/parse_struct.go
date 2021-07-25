package main

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"github.com/tealeg/xlsx"
	"regexp"
	"strings"
)

// 涉及到的类型
const (
	TYPE_NONE = 0
	TYPE_INT32 = 1
	TYPE_INT64 = 2
	TYPE_STRING = 3
	TYPE_FLOAT = 4
	TYPE_DATE = 5
)

// 字段在excel中的行数
const (
	NAME_ROW = 0
	TYPE_ROW = 2
	SCOPE_ROW = 3
	DATA_BEGIN_ROW = 4
)

type FieldDesc struct {
	Name string			// 字段名
	FieldType int		// 字段类型
	ProtoType int		// 字段在proto中的类型
	RepeatedInCell bool	// 是否是通过分隔符分割的repeated
	RepeatedInCol bool	// 是否是配在多个列中的repeated
}

func (d *FieldDesc) IsRepeated() bool {
	return d.RepeatedInCell || d.RepeatedInCol
}

func (d *FieldDesc) GetProtoType() string {
	if d.ProtoType == TYPE_INT32 {
		return "int32"
	} else if d.ProtoType == TYPE_STRING {
		return "string"
	} else if d.ProtoType == TYPE_FLOAT {
		return "float"
	} else if d.ProtoType == TYPE_INT64 {
		return "int64"
	}
	return ""
}

// 一个excel解析后对应的字段结构
type FileDesc struct {
	StructName string
	Fields []*FieldDesc
}

// 解析结构
func ParseStruct(structName string, sheet *xlsx.Sheet) *FileDesc {
	fileDesc := &FileDesc{StructName: structName}

	// 逐列解析
	for col := 0; col < sheet.MaxCol; col++ {
		parseField(col, sheet, fileDesc)
	}
	return fileDesc
}


func parseField(col int, sheet *xlsx.Sheet, fileDesc *FileDesc) {
	// fieldScope
	scope := strings.TrimSpace(sheet.Cell(SCOPE_ROW, col).String())
	if scope != "all" && scope != "server" {
		return
	}

	// fieldName
	fieldName := strcase.ToSnake(strings.TrimSpace(sheet.Cell(NAME_ROW, col).String()))

	// fieldType
	colType := strings.TrimSpace(sheet.Cell(TYPE_ROW, col).String())
	re := regexp.MustCompile("(int|float|string|time)(\\[\\])?")
	groups := re.FindStringSubmatch(colType)
	if len(groups) < 2 {
		fmt.Println("wrong type:", fileDesc.StructName, colType, col)
		return
	}
	t := groups[1]
	if t == "" {
		fmt.Println("wrong type: ", colType)
		return
	}
	fieldType := TYPE_NONE
	protoType := TYPE_NONE
	if t == "int" {
		fieldType = TYPE_INT32
		protoType = TYPE_INT32
	} else if t == "float" {
		fieldType = TYPE_FLOAT
		protoType = TYPE_FLOAT
	} else if t == "string" {
		fieldType = TYPE_STRING
		protoType = TYPE_STRING
	} else if t == "time" {
		fieldType = TYPE_DATE
		protoType = TYPE_INT64
	}

	// isRepeated
	// 这里先判断是否是int[]这种配置
	repeatedInCell := false
	if groups[2] != "" {
		repeatedInCell = true
	}
	// 判断是否是多列重名的数组配法
	repeatedInCol := false
	for i := 0; i < len(fileDesc.Fields); i++ {
		if fileDesc.Fields[i].Name == fieldName {
			fileDesc.Fields[i].RepeatedInCol = true
			repeatedInCol = true
		}
	}

	// 添加字段
	if !repeatedInCol {
		fileDesc.Fields = append(fileDesc.Fields, &FieldDesc{
			Name: fieldName,
			FieldType: fieldType,
			ProtoType: protoType,
			RepeatedInCell: repeatedInCell,
			RepeatedInCol: repeatedInCol,
		})
	}
}
