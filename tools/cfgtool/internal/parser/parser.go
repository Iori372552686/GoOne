package parser

import (
	"path/filepath"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/xuri/excelize/v2"
)

func ParseFiles(files ...string) error {
	for _, file := range files {
		logx.Infof("解析文件: %s\n", filepath.Base(file))
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
			logx.Warnf("%s没有定义生成表\n", fileName)
			return nil
		}
		logx.Errorf("获取生成表失败:%s\n", err.Error())
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
					return uerror.New(1, -1, "%s枚举表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTable(file, strs[1], domain.TypeOfEnum, "", data, nil)
			case "@struct":
				pos := strings.Index(strs[1], ":")
				data, err := fp.GetRows(strs[1][:pos])
				if err != nil {
					return uerror.New(1, -1, "%s结构表不存在%s  %v", fileName, strs[0], err.Error())
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

func buildField(tab *base.Table, col int) (*base.Field, error) {
	rawType := tab.Rows[2][col]
	elemType, arrayDepth := manager.SplitArrayType(rawType)
	if arrayDepth > domain.MaxArrayDepth {
		return nil, errs.New(tab.FileName, tab.Sheet, tab.Rows[1][col], 3, "类型错误", "数组最大仅支持到%d维: %s", domain.MaxArrayDepth, rawType)
	}

	typeOf := manager.GetTypeOf(elemType)
	convFunc := manager.GetConvFunc(elemType)
	if typeOf == domain.TypeOfBase && convFunc == nil {
		return nil, errs.New(tab.FileName, tab.Sheet, tab.Rows[1][col], 3, "类型错误", "未识别的类型: %s", elemType)
	}

	return &base.Field{
		Type: &base.Type{
			Name:       manager.GetConvType(elemType),
			TypeOf:     typeOf,
			ValueOf:    manager.GetValueOf(rawType),
			ArrayDepth: arrayDepth,
		},
		Name:     tab.Rows[1][col],                               //base.ToCamelCase(tab.Rows[1][col]),去掉了小驼峰规则需求
		Desc:     strings.ReplaceAll(tab.Rows[0][col], "\n", ""), // 字段描述,自动去掉换行，避免生成proto时出错
		Position: col,
		ConvFunc: convFunc,
	}, nil
}

func parseStruct(tab *base.Table) {
	// 表头至少需要 3 行：描述行 / 字段名行 / 类型行
	if len(tab.Rows) < 3 {
		logx.Warnf("结构表%s(%s)表头不足3行，已跳过\n", tab.Sheet, tab.FileName)
		return
	}

	st := manager.GetOrNewStruct(tab.FileName, tab.Sheet, tab.Type)
	for i, val := range tab.Rows[2] {
		if i >= len(tab.Rows[0]) || len(val) <= 0 || len(tab.Rows[0][i]) <= 0 {
			continue
		}

		field, err := buildField(tab, i)
		if err != nil {
			logx.Errorf("%v", err)
			continue
		}
		st.AddField(field)
	}
	if len(tab.Rows) > 4 {
		for _, vals := range tab.Rows[4:] {
			for i, val := range vals {
				if len(val) <= 0 || val == "0" {
					continue
				}
				if i >= len(st.FieldList) {
					continue
				}
				st.Converts[vals[0]] = append(st.Converts[vals[0]], st.FieldList[i])
			}
		}
	}
	tab.Rows = nil
}

func parseConfig(tab *base.Table) {
	// 表头至少需要 4 行：描述行 / 字段名行 / 类型行 / 标记行
	if len(tab.Rows) < 4 {
		logx.Warnf("配置表%s(%s)表头不足4行，已跳过\n", tab.Sheet, tab.FileName)
		return
	}

	cfg := manager.GetOrNewConfig(tab.FileName, tab.Sheet, tab.Type)

	// 收集第四行标记为 "key"/"KEY" 的字段，用于自动生成主键索引
	var autoKeyFields []*base.Field

	for i, val := range tab.Rows[2] {
		if i >= len(tab.Rows[0]) || len(val) <= 0 || len(tab.Rows[0][i]) <= 0 {
			continue
		}

		// 第四行标记
		tag := ""
		if i < len(tab.Rows[3]) {
			tag = strings.TrimSpace(tab.Rows[3][i])
		}
		tagLower := strings.ToLower(tag)

		// "key" 标记的列：在任何 mode 下都包含（等同 all），且自动作为主键索引
		isKey := tagLower == "key"

		// 过滤配置模式：key 标记的字段视为 all
		if !isKey {
			if domain.ConfMode != "all" && tag != "all" {
				if domain.ConfMode != tag || tag == "" {
					continue
				}
			}
		}

		field, err := buildField(tab, i)
		if err != nil {
			logx.Errorf("%v", err)
			continue
		}
		cfg.AddField(field)

		if isKey {
			autoKeyFields = append(autoKeyFields, field)
		}
	}

	// 默认索引
	cfg.AddIndex(&base.Index{
		Name: "List",
		Type: &base.Type{TypeOf: domain.TypeOfBase, ValueOf: domain.ValueOfList},
	})

	// ---- 自动主键索引（来自第四行 key 标记） ----
	if len(autoKeyFields) > 0 {
		indexName := ""
		for _, f := range autoKeyFields {
			indexName += f.Name
		}
		cfg.AddIndex(&base.Index{
			Name: indexName,
			Type: &base.Type{
				Name:    base.FieldList(autoKeyFields).GetIndexName(),
				TypeOf:  base.Ifelse(len(autoKeyFields) > 1, int(domain.TypeOfStruct), int(domain.TypeOfBase)),
				ValueOf: domain.ValueOfMap,
			},
			List: autoKeyFields,
		})
		// 同时设置 MapRules 以便 JSON 输出为 map 格式
		if tab.MapRules == "" {
			tab.MapRules = "key:" + indexName
		}
		logx.Infof("[%s/%s] 自动主键索引: %s", tab.FileName, tab.Sheet, indexName)
	}

	// ---- 手动索引（来自生成表 map:字段名 规则） ----
	for _, val := range tab.Rules {
		strs := strings.Split(val, ":")
		keys := []*base.Field{}
		for _, field := range strings.Split(strs[1], ",") {
			key := cfg.Fields[field]
			if key != nil {
				keys = append(keys, cfg.Fields[field])
			}
		}

		if len(keys) == 0 || strs[0] != "map" {
			continue
		}

		tab.MapRules = val
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
