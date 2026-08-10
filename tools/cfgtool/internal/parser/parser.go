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
		logx.Parsef("解析文件: %s", filepath.Base(file))
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

	// 解析功能名（从 xlsx 文件名 @ 前段取）。
	// 仅当文件名含 @（chinese 非空）时启用新命名体系，feature=功能名；
	// 无 @ 的旧式文件 feature 保持空串，走兼容路径（file 用文件基名，生成代码沿用 Name 推导）。
	feature, chinese := base.ParseFileFeature(fileName)
	if chinese == "" {
		feature = ""
	}

	// 读取「生成表」规则清单
	rows, err := fp.GetRows("生成表")
	if err != nil {
		if _, ok := err.(excelize.ErrSheetNotExist); ok {
			// 无生成表：把每个数据 sheet 自动注册为 config
			// （跳过以 _ 开头的辅助 sheet）
			return autoRegisterConfigs(fp, fileName, feature)
		}
		logx.Errorf("获取生成表失败:%s\n", err.Error())
		return uerror.New(1, -1, "获取生成表失败:%s", err.Error())
	}

	// 解析生成表（声明额外规则）；未在生成表里声明的数据 sheet 仍自动注册为 config。
	declaredSheets := map[string]bool{}
	for _, items := range rows {
		for _, val := range items {
			if len(val) <= 0 {
				continue
			}
			strs := strings.Split(val, "|")
			rule := strs[0]
			// file 默认 = 文件基名（无 @ 的旧式文件）；有 @ 时用功能名。
			// 兼容旧语法 @config:filename —— :filename 仍可覆盖输出文件名（= 功能名/proto 分桶），
			// 但不会覆盖 feature（Go 子目录/package 仍由文件名决定）。
			file := base.ProtoFileName(feature)
			if feature == "" {
				file = strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
			}
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

			   新命名体系（推荐）：xlsx 文件名与 sheet 名用 @ 拆分功能名/表名，
			   普通配置表可不写生成表，工具会自动把数据 sheet 注册为 config。
			   生成表仅用于声明额外规则（如 group 索引、struct/enum），声明的 sheet 不再自动注册。
			*/
			switch strings.ToLower(rule) {
			case "e":
				enum := manager.GetOrNewEnum(strs[2])
				enum.FileName = file
				enum.Feature = feature
				enum.AddValue(strs...)
			case "@enum":
				table, _ := base.ParseSheetTable(strs[1])
				data, err := fp.GetRows(strs[1])
				if err != nil {
					return uerror.New(1, -1, "%s枚举表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTableFull(file, strs[1], domain.TypeOfEnum, "", data, nil, feature, table)
				declaredSheets[strs[1]] = true
			case "@struct":
				pos := strings.Index(strs[1], ":")
				table, _ := base.ParseSheetTable(strs[1][:pos])
				data, err := fp.GetRows(strs[1][:pos])
				if err != nil {
					return uerror.New(1, -1, "%s结构表不存在%s  %v", fileName, strs[0], err.Error())
				}
				manager.AddTableFull(file, strs[1], domain.TypeOfStruct, strs[1][pos+1:], data, nil, feature, table)
				declaredSheets[strs[1][:pos]] = true
			case "@config":
				pos := strings.Index(strs[1], ":")
				data, err := fp.GetRows(strs[1][:pos])
				if err != nil {
					return uerror.New(1, -1, "%s配置表不存在%s  %v", fileName, strs[0], err.Error())
				}
				table, _ := base.ParseSheetTable(strs[1][:pos])
				manager.AddTableFull(file, strs[1], domain.TypeOfConfig, strs[1][pos+1:], data, base.Suffix(strs, 2), feature, table)
				declaredSheets[strs[1][:pos]] = true
			}
		}
	}

	// 有生成表时，未声明的数据 sheet 仍自动注册为 config（混合模式）
	return autoRegisterConfigsExcluding(fp, fileName, feature, declaredSheets)
}

// autoRegisterConfigs 在 xlsx 没有「生成表」sheet 时，自动把所有数据 sheet 注册为 config。
// sheet 名支持 @ 语法：掉落组表@group -> 表名=group；无 @ 则表名=sheet 名本身。
// 类型名 = ConfigTypeName(feature, table)，如 drop+group -> DropGroupConfig。
// 跳过以 _ 开头的辅助 sheet（如 _说明）。
func autoRegisterConfigs(fp *excelize.File, fileName, feature string) error {
	return autoRegisterConfigsExcluding(fp, fileName, feature, nil)
}

// autoRegisterConfigsExcluding 自动注册数据 sheet 为 config，但跳过 exclude 里已声明的 sheet。
// 用于混合模式：生成表声明的 sheet 走规则，其余 sheet 自动注册。
func autoRegisterConfigsExcluding(fp *excelize.File, fileName, feature string, exclude map[string]bool) error {
	sheets := fp.GetSheetList()
	if len(sheets) == 0 {
		logx.Warnf("%s没有任何sheet，已跳过\n", fileName)
		return nil
	}

	any := false
	for _, sheet := range sheets {
		// 跳过辅助 sheet（_ 开头）、生成表本身、已在生成表声明的 sheet
		if strings.HasPrefix(sheet, "_") || sheet == "生成表" || exclude[sheet] {
			continue
		}
		data, err := fp.GetRows(sheet)
		if err != nil {
			logx.Warnf("%s读取sheet[%s]失败：%v\n", fileName, sheet, err)
			continue
		}
		if len(data) < 4 {
			// 配置表至少需要 4 行表头（描述/字段名/类型/标记）
			logx.Warnf("%s sheet[%s]表头不足4行，已跳过\n", fileName, sheet)
			continue
		}
		table, _ := base.ParseSheetTable(sheet)
		typeName := base.ConfigTypeName(feature, table)
		// file：有 @ 时=功能名；无 @ 时=文件基名（旧式兼容）
		file := base.ProtoFileName(feature)
		if feature == "" {
			file = strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
		}
		manager.AddTableFull(file, sheet, domain.TypeOfConfig, typeName, data, nil, feature, table)
		logx.Parsef("[%s/%s] 自动注册配置: %s", file, sheet, typeName)
		any = true
	}
	if !any && len(exclude) == 0 {
		// 仅在没有生成表声明时提示——sheet 全部由「生成表」声明（如 @enum/@struct）
		// 的文件是正常形态，没有可自动注册的 sheet 属预期行为。
		logx.Warnf("%s没有可注册的数据sheet\n", fileName)
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
			enum.Feature = tab.Feature
			enum.Sheet = tab.Sheet
			enum.AddValue(strs...)
		}
	}
}

func buildField(tab *base.Table, col int) (*base.Field, error) {
	rawType := tab.Rows[2][col]

	// map[K]V 分支：优先识别，单独走结构化构造
	if keyName, valName, ok := manager.SplitMapType(rawType); ok {
		return buildMapField(tab, col, rawType, keyName, valName)
	}

	elemType, arrayDepth := manager.SplitArrayType(rawType)
	if arrayDepth > domain.MaxArrayDepth {
		return nil, errs.New(tab.FileName, tab.Sheet, tab.Rows[1][col], 3, "类型错误", "数组最大仅支持到%d维: %s", domain.MaxArrayDepth, rawType)
	}

	// 外部 proto 类型（pb.XXX）分支：识别后走专用构造
	if short, ok := manager.SplitExternalType(elemType); ok {
		return buildExternalField(tab, col, short, arrayDepth)
	}

	typeOf := manager.GetTypeOf(elemType)
	convFunc := manager.GetConvFunc(elemType)
	if typeOf == domain.TypeOfBase && convFunc == nil {
		return nil, errs.New(tab.FileName, tab.Sheet, tab.Rows[1][col], 3, "类型错误", "未识别的类型: %s", elemType)
	}

	container := domain.ContainerSingle
	if arrayDepth > 0 {
		container = domain.ContainerArray
	}

	return &base.Field{
		Type: &base.Type{
			Name:       manager.GetConvType(elemType),
			TypeOf:     typeOf,
			ValueOf:    manager.GetValueOf(rawType),
			ArrayDepth: arrayDepth,
			Container:  container,
		},
		Name:     tab.Rows[1][col],                               //base.ToCamelCase(tab.Rows[1][col]),去掉了小驼峰规则需求
		Desc:     strings.ReplaceAll(tab.Rows[0][col], "\n", ""), // 字段描述,自动去掉换行，避免生成proto时出错
		Position: col,
		ConvFunc: convFunc,
	}, nil
}

// buildExternalField 构造引用外部 proto message/enum 的字段。
// pb.XXX 语法解析后，XXX 在 externalMsgMgr/externalEnumMgr 中查表。
// 能力与内置 struct/enum 对齐：支持单值、多维数组（map 由 buildMapField 单独处理 V）。
// proto 生成时用裸短名（同包 g1.protocol），IsExternal 标记驱动 import 收集与反射赋值。
func buildExternalField(tab *base.Table, col int, short string, arrayDepth int) (*base.Field, error) {
	fieldName := tab.Rows[1][col]

	isMsg := manager.IsExternalMsg(short)
	isEnum := manager.IsExternalEnum(short)
	if !isMsg && !isEnum {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
			"外部proto类型未注册: pb.%s（请检查 -proto-src 目录是否包含定义该类型的 .proto）", short)
	}

	typeOf := domain.TypeOfStruct
	var convFunc func(string) interface{}
	if isEnum {
		typeOf = domain.TypeOfEnum
		// 外部 enum：优先按短名查内置 enumMgr 的中文枚举转换（若同名注册过），
		// 否则按 int32 转换（proto enum 在 dynamic 里就是 int32）
		convFunc = manager.GetConvFunc(short)
		if convFunc == nil {
			convFunc = manager.GetConvFunc("int32")
		}
	}

	container := domain.ContainerSingle
	if arrayDepth > 0 {
		container = domain.ContainerArray
	}

	return &base.Field{
		Type: &base.Type{
			Name:       short, // proto 字段类型用裸短名（同包 g1.protocol）
			TypeOf:     typeOf,
			ValueOf:    manager.GetValueOf(rawTypePlaceholder(arrayDepth)),
			ArrayDepth: arrayDepth,
			Container:  container,
		},
		Name:       fieldName,
		Desc:       strings.ReplaceAll(tab.Rows[0][col], "\n", ""),
		Position:   col,
		ConvFunc:   convFunc,
		IsExternal: true,
	}, nil
}

// rawTypePlaceholder 生成一个仅用于 GetValueOf 判定 List/Base 的占位类型字符串。
// 外部类型的 ValueOf 由数组维度决定，不依赖具体类型名。
func rawTypePlaceholder(arrayDepth int) string {
	if arrayDepth == 0 {
		return "int32"
	}
	prefix := ""
	for i := 0; i < arrayDepth; i++ {
		prefix += "[]"
	}
	return prefix + "int32"
}

// buildMapField 构造 map[K]V 字段。
// 本期能力边界：
//   - K：标量（int*/uint*/string/bool），不接受 enum/float/double/struct/array（受 protobuf3 规范约束）
//   - V：标量/枚举/结构体，不接受数组、嵌套 map
func buildMapField(tab *base.Table, col int, rawType, keyName, valName string) (*base.Field, error) {
	fieldName := tab.Rows[1][col]

	// ---- 校验 K ----
	keyConvName := manager.GetConvType(keyName)
	keyConvFunc := manager.GetConvFunc(keyName)
	keyTypeOf := manager.GetTypeOf(keyName)
	if keyTypeOf != domain.TypeOfBase || keyConvFunc == nil {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
			"map 的 key 仅支持标量(int/uint/string/bool): %s", keyName)
	}
	_ = keyConvFunc // 转换函数在 data_gen 阶段按 KeyType.Name 现查，避免 Type 持有函数
	if !isValidMapKey(keyConvName) {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
			"map 的 key 不允许使用浮点/枚举/结构体/数组(protobuf3规范): %s", keyName)
	}

	// ---- 校验 V ----
	// V 可能是 pb.XXX 外部类型，剥前缀取短名
	valShort, isExternalV := manager.SplitExternalType(valName)
	valLookupName := valName
	valNameForProto := manager.GetConvType(valName)
	if isExternalV {
		if !manager.IsExternalMsg(valShort) && !manager.IsExternalEnum(valShort) {
			return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
				"外部proto类型未注册: pb.%s", valShort)
		}
		valLookupName = valShort
		valNameForProto = valShort // proto 用裸短名
	}
	valConvFunc := manager.GetConvFunc(valLookupName)
	valTypeOf := manager.GetTypeOf(valLookupName)
	if valTypeOf == domain.TypeOfBase && valConvFunc == nil && !isExternalV {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误", "未识别的类型: %s", valName)
	}
	// V 不允许数组/嵌套 map
	if _, _, isMap := manager.SplitMapType(valName); isMap {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
			"map 的 value 不支持嵌套 map: %s", valName)
	}
	if _, depth := manager.SplitArrayType(valName); depth > 0 {
		return nil, errs.New(tab.FileName, tab.Sheet, fieldName, 3, "类型错误",
			"map 的 value 不支持数组(本期): %s", valName)
	}

	return &base.Field{
		Type: &base.Type{
			Name:      valNameForProto,
			TypeOf:    valTypeOf,
			Container: domain.ContainerMap,
			KeyType: &base.Type{
				Name:   keyConvName,
				TypeOf: domain.TypeOfBase,
			},
		},
		Name:       fieldName,
		Desc:       strings.ReplaceAll(tab.Rows[0][col], "\n", ""),
		Position:   col,
		ConvFunc:   valConvFunc, // V 的标量转换函数；结构体 V 由 data_gen 另走 parseStructMessage
		IsExternal: isExternalV, // map value 引用外部类型时标记
	}, nil
}

// isValidMapKey 判定归一化后的标量类型名是否允许作为 map key（protobuf3 规范）。
func isValidMapKey(protoTypeName string) bool {
	switch protoTypeName {
	case "int32", "int64", "uint32", "uint64", "string", "bool":
		return true
	}
	return false
}

func parseStruct(tab *base.Table) {
	// 表头至少需要 3 行：描述行 / 字段名行 / 类型行
	if len(tab.Rows) < 3 {
		logx.Warnf("结构表%s(%s)表头不足3行，已跳过\n", tab.Sheet, tab.FileName)
		return
	}

	st := manager.GetOrNewStruct(tab.FileName, tab.Sheet, tab.Type, tab.Feature, tab.Table)
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

	cfg := manager.GetOrNewConfig(tab.FileName, tab.Sheet, tab.Type, tab.Feature, tab.Table)

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
		logx.Parsef("[%s/%s] 自动主键索引: %s", tab.FileName, tab.Sheet, indexName)
	}

	// ---- 手动索引（来自生成表 map:/group: 字段名 规则） ----
	for _, val := range tab.Rules {
		strs := strings.Split(val, ":")
		ruleKind := strings.ToLower(strs[0])
		if ruleKind != "map" && ruleKind != "group" {
			continue
		}
		keys := []*base.Field{}
		for _, field := range strings.Split(strs[1], ",") {
			key := cfg.Fields[field]
			if key != nil {
				keys = append(keys, cfg.Fields[field])
			}
		}

		if len(keys) == 0 {
			// 索引引用的字段被当前 gen 模式过滤（如 group:Groupid 而 Groupid 标了
			// server/client）——不静默丢弃，显式提示，避免生成代码缺索引方法。
			logx.Warnf("[%s/%s] 索引规则 %s 引用的字段在当前模式(%s)下不存在，索引未生成\n",
				tab.FileName, tab.Sheet, val, domain.ConfMode)
			continue
		}

		// map 索引影响 JSON 输出格式（map[key]obj）；group 不影响（仍是 list）
		if ruleKind == "map" {
			tab.MapRules = val
		}
		valueOf := base.Ifelse(ruleKind == "map", int(domain.ValueOfMap), int(domain.ValueOfGroup))
		switch len(strs) {
		case 2:
			cfg.AddIndex(&base.Index{
				Name: strings.ReplaceAll(strs[1], ",", ""),
				Type: &base.Type{
					Name:    base.FieldList(keys).GetIndexName(),
					TypeOf:  base.Ifelse(len(keys) > 1, int(domain.TypeOfStruct), int(domain.TypeOfBase)),
					ValueOf: valueOf,
				},
				List: keys,
			})
		case 3:
			cfg.AddIndex(&base.Index{
				Name: strs[2],
				Type: &base.Type{
					Name:    base.FieldList(keys).GetIndexName(),
					TypeOf:  base.Ifelse(len(keys) > 1, int(domain.TypeOfStruct), int(domain.TypeOfBase)),
					ValueOf: valueOf,
				},
				List: keys,
			})
		}
	}
}
