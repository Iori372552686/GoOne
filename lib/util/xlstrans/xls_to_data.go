package main

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/tealeg/xlsx"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

type XlsToData struct {
	FileDesc *FileDesc
	Sheet    *xlsx.Sheet

	RootMsgDesc *desc.MessageDescriptor
	ItemMsgDesc *desc.MessageDescriptor
	RootMsg     *dynamic.Message

	keys map[int64]bool
}

func (p *XlsToData) Parse(fileDesc *FileDesc, sheet *xlsx.Sheet) {
	p.FileDesc = fileDesc
	p.Sheet = sheet
	p.keys = make(map[int64]bool)

	p.openFile()
	p.parse()
	p.writeDataToFile()
	p.writeReadableDataToFile()
}

func (p *XlsToData) openFile() {
	// 解析proto文件，用于后面反射
	var paser protoparse.Parser
	descs, err := paser.ParseFiles("proto/dataconfig_" + p.FileDesc.StructName + ".proto")
	if err != nil {
		fmt.Println("parse proto file error", err)
		return
	}

	// 获取proto的desc
	d := descs[0]
	p.RootMsgDesc = d.FindMessage("gamedata." + p.FileDesc.StructName + "Array")
	if p.RootMsgDesc == nil {
		fmt.Println("find message desc nil")
		return
	}
	p.ItemMsgDesc = d.FindMessage("gamedata." + p.FileDesc.StructName)
	if p.ItemMsgDesc == nil {
		fmt.Println("find item message desc nil")
		return
	}
}

func (p *XlsToData) parse() {
	// 通过protobuf的反射创建结构体
	p.RootMsg = dynamic.NewMessage(p.RootMsgDesc)
	if p.RootMsg == nil {
		fmt.Println("create proto message nil")
		return
	}

	for i := DATA_BEGIN_ROW; i < len(p.Sheet.Rows); i += 1 {
		id := strings.TrimSpace(p.Sheet.Cell(i, 0).String())
		if id == "" {
			//fmt.Println("id none| ", p.structName, " |row: ", i)
			continue
		}
		p.parseLine(i)
	}
}

func (p *XlsToData) parseLine(row int) {
	item := dynamic.NewMessage(p.ItemMsgDesc)
	if item == nil {
		fmt.Println("create item message nil")
		return
	}

	// 逐行解析字段
	for i := 0; i < p.Sheet.MaxCol; i += 1 {
		p.parseField(row, i, item)
	}

	// 添加一条记录到protobuf结构体
	p.RootMsg.AddRepeatedFieldByName("items", item)
}

func (p *XlsToData) parseField(row, col int, item *dynamic.Message) {
	// fieldScope
	scope := strings.TrimSpace(p.Sheet.Cell(SCOPE_ROW, col).String())
	if scope != "all" && scope != "server" {
		return
	}

	// fieldName
	fieldName := strcase.ToSnake(strings.TrimSpace(p.Sheet.Cell(NAME_ROW, col).String()))
	var fieldDesc *FieldDesc = nil
	for _, v := range p.FileDesc.Fields {
		if fieldName == v.Name {
			fieldDesc = v
			break
		}
	}
	if fieldDesc == nil {
		fmt.Println("get field desc error", col)
		return
	}

	// 根据类型将字段塞入结构体
	vVec := make([]string, 0)
	content := strings.TrimSpace(p.Sheet.Cell(row, col).String())
	if fieldDesc.RepeatedInCell {
		vtmp := strings.Split(content, "|")
		for _, v := range vtmp {
			vVec = append(vVec, strings.Split(v, ",")...)
		}
	} else {
		vVec = append(vVec, content)
	}

	for _, v := range vVec {
		if fieldDesc.FieldType == TYPE_INT32 {
			res, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				res = 0
			}
			// 检查重复key
			if col == 0 {
				if _, exist := p.keys[res]; exist {
					fmt.Printf("key duplicated in %v, row = %v, key = %v\n", p.FileDesc.StructName, row, res)
				} else {
					p.keys[res] = true
				}
			}

			if fieldDesc.IsRepeated() {
				item.AddRepeatedFieldByName(fieldName, int32(res))
			} else {
				item.SetFieldByName(fieldName, int32(res))
			}
		} else if fieldDesc.FieldType == TYPE_FLOAT {
			res, err := strconv.ParseFloat(v, 32)
			if err != nil {
				res = 0.0
			}
			if fieldDesc.IsRepeated() {
				item.AddRepeatedFieldByName(fieldName, float32(res))
			} else {
				item.SetFieldByName(fieldName, float32(res))
			}
		} else if fieldDesc.FieldType == TYPE_STRING {
			if fieldDesc.IsRepeated() {
				item.AddRepeatedFieldByName(fieldName, v)
			} else {
				item.SetFieldByName(fieldName, v)
			}
		} else if fieldDesc.FieldType == TYPE_DATE {
			timeZone, _ := time.LoadLocation("Asia/Chongqing")
			tmUnix := int64(0)
			if len(v) == 8 { // 有可能只配不配置日期: 13:21:02
				vect := strings.Split(v, ":")
				if len(vect) == 3 {
					hour, _ := strconv.Atoi(vect[0])
					minute, _ := strconv.Atoi(vect[1])
					second, _ := strconv.Atoi(vect[2])
					tmUnix = int64(hour*3600 + minute*60 + second)
					//fmt.Println(hour, minute, second, tmUnix)
				}
			} else {
				tm, err := time.ParseInLocation("2006-01-02 15:04:05", v, timeZone)
				tmUnix = tm.Unix()
				if err != nil {
					tmUnix = 0
					if v != "" {
						fmt.Println("date time error", p.FileDesc.StructName, v, row, col)
					}
				}
			}
			if fieldDesc.IsRepeated() {
				item.AddRepeatedFieldByName(fieldName, tmUnix)
			} else {
				item.SetFieldByName(fieldName, tmUnix)
			}
		} else {
			fmt.Println("no type find")
		}
	}
}

func (p *XlsToData) writeDataToFile() {
	fileName := "data/dataconfig_" + p.FileDesc.StructName + ".data"
	data, e := p.RootMsg.Marshal()
	if e != nil {
		fmt.Println("marshal error", e)
	}

	err := ioutil.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Println("write data file err", err)
	}
}

func (p *XlsToData) writeReadableDataToFile() {
	fileName := targetDataPath + "/dataconfig_" + p.FileDesc.StructName + ".config"
	data, e := p.RootMsg.MarshalTextIndent()

	if e != nil {
		fmt.Println("marshal error", e)
	}

	err := ioutil.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Println("write config file err", err)
	}
}
