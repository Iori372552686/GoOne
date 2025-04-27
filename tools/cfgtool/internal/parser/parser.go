package parser

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"path/filepath"
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/xuri/excelize/v2"
)

func ParseFiles(files ...string) error {
	for _, file := range files {
		fmt.Printf("解析文件: %s\n", filepath.Base(file))
		if err := parseTable(file); err != nil {
			return err
		}
	}
	// 解析
	for _, en := range manager.GetTableList(domain.TypeOfEnum) {
		parseEnum(en)
	}
	for _, item := range manager.GetTableList(domain.TypeOfStruct) {
		parseStruct(item)
	}
	for _, item := range manager.GetTableList(domain.TypeOfConfig) {
		parseConfig(item)
	}
	parseReference()
	return nil
}

func parseTable(fileName string) error {
	fp, err := excelize.OpenFile(fileName)
	if err != nil {
		return uerror.New(1, -1, "打开文件失败:%s", err.Error())
	}
	defer fp.Close()

	// 读取所有数据
	rows, err := fp.GetRows("生成表")
	if err != nil {
		if _, ok := err.(excelize.ErrSheetNotExist); ok {
			fmt.Printf("%s没有定义生成表\n", fileName)
			return nil
		}
		fmt.Printf("获取生成表失败:%s\n", err.Error())
		return uerror.New(1, -1, "获取生成表失败:%s", err.Error())
	}
	file := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))

	// 解析生成表
	for _, items := range rows {
		for _, val := range items {
			if len(val) <= 0 {
				continue
			}
			strs := strings.Split(val, "|")
			rule := strs[0]
			pos := strings.Index(strs[0], ":")
			if pos > 0 {
				file = strs[0][pos+1:]
				rule = strs[0][:pos]
			}
			/*
			   @config[:filename]|sheet:结构名|map:字段名[,字段名]:别名|group:字段名[,字段名]:别名
			   @struct[:filename]|sheet:结构名
			   @enum[:filename]|sheet
			   E|道具类型-金币|PropertType|Coin|1
			*/
			switch strings.ToLower(rule) {
			case "e":
				enum := manager.GetOrNewEnum(strs[2])
				enum.FileName = file
				enum.AddValue(strs...)
			case "@enum":
				data, err := fp.GetRows(strs[1])
				if err != nil {
					return uerror.New(1, -1, "%s配置表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTable(file, strs[1], domain.TypeOfEnum, "", data, nil)
			case "@struct":
				pos := strings.Index(strs[1], ":")
				data, err := fp.GetRows(strs[1][:pos])
				if err != nil {
					return uerror.New(1, -1, "%s配置表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTable(file, strs[1], domain.TypeOfStruct, strs[1][pos+1:], data, nil)
			case "@config":
				pos := strings.Index(strs[1], ":")
				data, err := fp.GetRows(strs[1][:pos])
				if err != nil {
					return uerror.New(1, -1, "%s配置表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTable(file, strs[1], domain.TypeOfConfig, strs[1][pos+1:], data, base.Suffix(strs, 2))
			}
		}
	}
	return nil
}

func parseEnum(tab *base.Table) {
	for _, vals := range tab.Rows {
		for _, val := range vals {
			if !strings.HasPrefix(val, "E|") && !strings.HasPrefix(val, "e|") {
				continue
			}

			strs := strings.Split(val, "|")
			enum := manager.GetOrNewEnum(strs[2])
			enum.FileName = tab.FileName
			enum.Sheet = tab.Sheet
			enum.AddValue(strs...)
		}
	}
}

func parseStruct(tab *base.Table) {
	st := manager.GetOrNewStruct(tab.FileName, tab.Sheet, tab.Type)
	for i, val := range tab.Rows[1] {
		if len(val) <= 0 || len(tab.Rows[0][i]) <= 0 {
			continue
		}
		vType := strings.TrimPrefix(val, "[]")
		st.AddField(&base.Field{
			Type: &base.Type{
				Name:    manager.GetConvType(vType),
				TypeOf:  manager.GetTypeOf(vType),
				ValueOf: manager.GetValueOf(val),
			},
			Name:     tab.Rows[0][i],
			Desc:     tab.Rows[2][i],
			Position: i,
			ConvFunc: manager.GetConvFunc(vType),
		})
	}
	for _, vals := range tab.Rows[3:] {
		for i, val := range vals {
			if len(val) <= 0 || val == "0" {
				continue
			}
			st.Converts[vals[0]] = append(st.Converts[vals[0]], st.FieldList[i])
		}
	}
	tab.Rows = nil
}

func parseConfig(tab *base.Table) {
	cfg := manager.GetOrNewConfig(tab.FileName, tab.Sheet, tab.Type)
	for i, val := range tab.Rows[1] {
		if len(val) <= 0 || len(tab.Rows[0][i]) <= 0 {
			continue
		}
		vType := strings.TrimPrefix(val, "[]")
		cfg.AddField(&base.Field{
			Type: &base.Type{
				Name:    manager.GetConvType(vType),
				TypeOf:  manager.GetTypeOf(vType),
				ValueOf: manager.GetValueOf(val),
			},
			Name:     tab.Rows[0][i],
			Desc:     tab.Rows[2][i],
			Position: i,
			ConvFunc: manager.GetConvFunc(vType),
		})
	}

	// 默认索引
	cfg.AddIndex(&base.Index{
		Name: "List",
		Type: &base.Type{TypeOf: domain.TypeOfBase, ValueOf: domain.ValueOfList},
	})
	// 解析索引   map:字段名[,字段名]:别名|group:字段名[,字段名]:别名
	for _, val := range tab.Rules {
		strs := strings.Split(val, ":")
		keys := []*base.Field{}
		for _, field := range strings.Split(strs[1], ",") {
			keys = append(keys, cfg.Fields[field])
		}
		switch len(strs) {
		case 2:
			cfg.AddIndex(&base.Index{
				Name: strings.ReplaceAll(strs[1], ",", ""),
				Type: &base.Type{
					Name:    base.FieldList(keys).GetIndexName(),
					TypeOf:  base.Ifelse(len(keys) > 1, int(domain.TypeOfStruct), int(domain.TypeOfBase)),
					ValueOf: base.Ifelse(strings.ToLower(strs[0]) == "map", int(domain.ValueOfMap), int(domain.ValueOfBase)),
				},
				List: keys,
			})
		case 3:
			cfg.AddIndex(&base.Index{
				Name: strs[2],
				Type: &base.Type{
					Name:    base.FieldList(keys).GetIndexName(),
					TypeOf:  base.Ifelse(len(keys) > 1, int(domain.TypeOfStruct), int(domain.TypeOfBase)),
					ValueOf: base.Ifelse(strings.ToLower(strs[0]) == "map", int(domain.ValueOfMap), int(domain.ValueOfBase)),
				},
				List: keys,
			})
		}
	}
}
