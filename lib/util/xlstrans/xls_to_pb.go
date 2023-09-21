/// 将解析出的结构体生成.proto文件

package main

import (
	"fmt"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"strconv"
)

type XlsToPb struct {
	fileDesc *FileDesc
	sheet    *xlsx.Sheet

	output string

	indentation int
	fieldIndex  int
}

func (t *XlsToPb) Trans(fileDesc *FileDesc, sheet *xlsx.Sheet) error {
	t.fileDesc = fileDesc
	t.sheet = sheet
	t.output = ""
	t.indentation = 0
	t.fieldIndex = 0

	t.layoutFileHeader()

	t.layoutStructHead()
	t.increaseIndentation()

	for _, v := range t.fileDesc.Fields {
		t.layoutOneField(v)
	}

	t.decreaseIndentation()
	t.layoutStructTail()

	t.layoutArray()

	t.write2File()

	return nil
}

func (t *XlsToPb) layoutFileHeader() {
	t.output += "/**\n"
	t.output += "* @brief:  这个文件是通过工具自动生成的，建议不要手动修改\n"
	t.output += "*/\n"
	t.output += "\n"
	t.output += "syntax = \"proto3\";\n"
	t.output += "\n"
	t.output += "package gamedata;\n"
}

func (t *XlsToPb) appendIndentation() {
	for i := 0; i < t.indentation; i += 1 {
		t.output += " "
	}
}

// message StructName {
func (t *XlsToPb) layoutStructHead() {
	t.output += "\n"
	t.appendIndentation()
	t.output += "message " + t.fileDesc.StructName + " {\n"
}

//		  repeated int FieldName1 = 1;
//	   string FieldName2 = 2;
func (t *XlsToPb) layoutOneField(fieldDesc *FieldDesc) {
	t.appendIndentation()
	if fieldDesc.IsRepeated() {
		t.output += "repeated "
	}
	t.output += fieldDesc.GetProtoType() + " " + fieldDesc.Name + " = " +
		strconv.Itoa(t.getAndAddFieldIndex()) + ";\n"
}

// }
func (t *XlsToPb) layoutStructTail() {
	t.appendIndentation()
	t.output += "}\n"
	t.output += "\n"
}

func (t *XlsToPb) getAndAddFieldIndex() int {
	t.fieldIndex += 1
	return t.fieldIndex
}

func (t *XlsToPb) increaseIndentation() {
	t.indentation += 4
}

func (t *XlsToPb) decreaseIndentation() {
	t.indentation -= 4
}

//	message StructNameArray {
//		  repeated StructName items = 1;
//	}
func (t *XlsToPb) layoutArray() {
	t.output += "message " + t.fileDesc.StructName + "Array {\n"
	t.output += "    repeated " + t.fileDesc.StructName + " items = 1;\n}\n"
}

func (t *XlsToPb) write2File() {
	data := []byte(t.output)
	err := ioutil.WriteFile(targetProtoPath+"/dataconfig_"+t.fileDesc.StructName+".proto", data, 0644)
	if err != nil {
		fmt.Println(err)
	}
}
