package service

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"strings"
)

// arrayDelimiters 是数组/Map 元素的层级分隔符表（类型无关）。
// 设计原则（鲁班式「纯层级化」）：
//   - 分隔符仅表达「第几维」，与元素是标量还是结构体无关 —— 标量与结构体同一维度用同一符号
//   - 层级递进：第1维 |（最细）→ 第2维 ; → 第3维 ^（最粗）
//   - 与结构体成员分隔符 ',' 和 map K-V 分隔符 ':' 完全正交，无歧义
//
// 示例：
//
//	[]int32        → 1|2|3
//	[][]int64      → 1|2;3|4
//	[]Reward       → 1,10|2,20        (结构体成员用 ',')
//	map[int32]Reward → 1:1,10|2:2,20  (K:V 用 ':', 元素间用 '|')
var arrayDelimiters = []string{"|", ";", "^"}

// mapFieldDelim 是 map 元素之间的分隔符。
// 取 ';'（高于结构体成员 ',' 与 repeated 内部 '|'），保证 value 是含 repeated 字段的
// 结构体时（如 pb.TexasGameEndInfo 的 hands/bests 用 '|'）不与元素分隔冲突。
const mapFieldDelim = ";"

func GenData() error {
	// 各格式成功生成的文件名列表（末尾汇总打印）
	var jsonFiles, confFiles, bytesFiles, luaFiles []string
	for _, cfg := range manager.GetConfigMap() {
		// 反射new一个对象
		ary := manager.NewProto(cfg.FileName, cfg.Name+"Ary")
		if ary == nil {
			return uerror.New(1, -1, "new %sAry is nil", cfg.Name)
		}

		// 几种结构加载xlsx数据
		tab := manager.GetTable(cfg.FileName, cfg.Sheet)
		needJson := len(domain.JsonPath) > 0
		needLua := len(domain.LuaPath) > 0
		needTableData := needJson || needLua
		useMap := tab.MapRules != ""

		// 确定 map 主键字段名列表（从 Config 的 Map 索引中取，而非 hard-code 第一个字段）
		var mapKeyFields []string
		if useMap {
			for _, idx := range cfg.IndexList {
				if idx.Type.ValueOf == domain.ValueOfMap && len(idx.List) > 0 {
					for _, f := range idx.List {
						mapKeyFields = append(mapKeyFields, f.Name)
					}
					break // 取第一个 Map 索引作为 JSON key
				}
			}
		}

		var tabAry []interface{}
		var tabMap map[string]interface{}
		if needTableData {
			if useMap {
				tabMap = make(map[string]interface{}, len(tab.Rows)-4)
			} else {
				tabAry = make([]interface{}, 0, len(tab.Rows)-4)
			}
		}

		for i, vals := range tab.Rows[4:] {
			rowNum := i + 5 // Excel可视行号
			item, err := configValue(cfg, rowNum, vals...)
			if err != nil {
				return err
			}

			if err := safeAddRepeatedFieldByName(ary, "Ary", item, cfg.FileName, cfg.Sheet, cfg.Name+"Ary", "Ary", rowNum); err != nil {
				return err
			}

			// JSON/Lua 不需要时跳过 dynamic -> 通用结构转换
			if !needTableData {
				continue
			}

			msg, ok := item.(*dynamic.Message)
			if !ok {
				continue
			}

			meta, ok := dynamicValueToInterface(msg).(map[string]interface{})
			if !ok {
				continue
			}

			if useMap {
				// 用实际配置的主键字段拼接 map key
				key := buildMapKey(meta, mapKeyFields)
				tabMap[key] = meta
			} else {
				tabAry = append(tabAry, meta)
			}
		}

		// save json
		if needJson {
			var source interface{}
			if useMap {
				source = tabMap
			} else {
				source = tabAry
			}
			bufStr, err := TransToArray2(source)
			if err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "生成错误", "JSON序列化失败")
			}
			if err := base.Save(domain.JsonPath, cfg.Name+".json", bufStr); err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "保存错误", "保存JSON失败")
			}
			jsonFiles = append(jsonFiles, cfg.Name+".json")
		}

		// save lua数据
		if needLua {
			var buf []byte
			var err error

			if tab.MapRules == "" {
				buf, err = MarshalToLuaConf(tabAry, tab.LuaRules)
			} else {
				buf, err = MarshalToLuaConf(tabMap, tab.LuaRules)
			}
			if err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "生成错误", "生成Lua数据失败")
			}
			if err := base.Save(domain.LuaPath, cfg.Name+".lua", buf); err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "保存错误", "保存Lua失败")
			}
			luaFiles = append(luaFiles, cfg.Name+".lua")
		}

		// 保存pb bytes数据
		if len(domain.BytesPath) > 0 {
			buf, err := ary.Marshal()
			if err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "生成错误", "序列化pb bytes失败")
			}
			if err := base.Save(domain.BytesPath, cfg.Name+".bytes", buf); err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "保存错误", "保存pb bytes失败")
			}
			bytesFiles = append(bytesFiles, cfg.Name+".bytes")
		}

		// 保存pb text数据
		if len(domain.TextPath) > 0 {
			buf, err := ary.MarshalTextIndent()
			if err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "生成错误", "序列化pb text失败")
			}
			if err := base.Save(domain.TextPath, cfg.Name+".conf", buf); err != nil {
				return errs.Wrap(err, cfg.FileName, cfg.Sheet, "", 0, "保存错误", "保存pb text失败")
			}
			confFiles = append(confFiles, cfg.Name+".conf")
		}

		// 保存ts数据
		if len(domain.TsPath) > 0 {
			/*	buf, err := MarshalTS(ary)  //todo :)
				if err != nil {
					return err
				}
				if err := base.Save(domain.TsPath, cfg.Name+".ts", buf); err != nil {
					return err
				}*/
		}
	}

	// 各格式汇总成功提示（仅打印本次实际生成且启用的格式），并列出生成的文件
	printDataSummary("JSON数据", jsonFiles, domain.JsonPath)
	printDataSummary("pbtext数据", confFiles, domain.TextPath)
	printDataSummary("pb bytes数据", bytesFiles, domain.BytesPath)
	printDataSummary("Lua数据", luaFiles, domain.LuaPath)

	// 注意：此处不再调用 manager.Clear()。
	// GenCode/GenCpp/GenNodeJs 在本函数之后执行，它们仍需读取 configMgr/structMgr；
	// Clear 统一由 main.run() 在所有 Gen* 完成后调用。
	return nil
}

// printDataSummary 打印某格式的数据生成汇总：数量 + 输出目录 + 每个文件名。
func printDataSummary(kind string, files []string, dir string) {
	if len(files) == 0 {
		return
	}
	logx.Successf("%s生成完成: %d 个文件 -> %s", kind, len(files), dir)
	for _, f := range files {
		logx.Infof("  - %s", f)
	}
}

func configValue(f *base.Config, rowIndex int, vals ...string) (interface{}, error) {
	// 反射new一个对象
	item := manager.NewProto(f.FileName, f.Name)
	if item == nil {
		return nil, errs.New(f.FileName, f.Sheet, "", rowIndex, "NewProto", "new %s is nil", f.Name)
	}

	for _, field := range f.FieldList {
		if field.Position >= len(vals) {
			break
		}
		if err := assignFieldValue(item, f.FileName, f.Sheet, f.Name, field, rowIndex, vals[field.Position]); err != nil {
			return nil, err
		}
	}

	//logx.Infof("已解析 %s/%s 行=%d", f.FileName, f.Sheet, rowIndex)
	return item, nil
}

func assignFieldValue(msg *dynamic.Message, file, sheet, msgName string, field *base.Field, rowIndex int, raw string) error {
	value, err := parseFieldValue(file, sheet, field, rowIndex, raw)
	if err != nil {
		return errs.Wrap(err, file, sheet, field.Name, rowIndex, "字段解析错误", "字段值: %q", raw)
	}
	if value == nil {
		return nil
	}

	if field.Type.ArrayDepth > 0 || field.Type.ValueOf == domain.ValueOfList {
		values, ok := value.([]interface{})
		if !ok {
			return errs.New(file, sheet, field.Name, rowIndex, "赋值错误", "数组字段解析结果不是切片: %T", value)
		}
		return safeSetRepeatedFieldByName(msg, field.Name, values, file, sheet, msgName, field.Name, rowIndex)
	}
	// map[K]V：作为 singular 字段整体 SetField（proto3 map 在 dynamic 里以 map 形式赋值）
	if field.Type.Container == domain.ContainerMap {
		return safeSetMapField(msg, field.Name, value, file, sheet, msgName, field.Name, rowIndex)
	}
	return safeSetField(msg, field.Name, value, file, sheet, msgName, field.Name, rowIndex)
}

func parseFieldValue(file, sheet string, field *base.Field, rowIndex int, raw string) (interface{}, error) {
	if strings.TrimSpace(raw) == "" {
		switch {
		case field.Type.Container == domain.ContainerMap:
			return nil, nil
		case field.Type.ArrayDepth > 0, field.Type.Container == domain.ContainerArray:
			return nil, nil
		case field.Type.TypeOf == domain.TypeOfStruct:
			logx.Warnf("[%s/%s] 字段=%s 行=%d 值为空(结构体) 跳过\n", file, sheet, field.Name, rowIndex)
			return nil, nil
		default:
			return field.ConvFunc(raw), nil
		}
	}

	// map[K]V 分支
	if field.Type.Container == domain.ContainerMap {
		return parseMapValue(file, sheet, field, rowIndex, raw)
	}

	if field.Type.ArrayDepth > 0 {
		return parseArrayValue(file, sheet, field, rowIndex, raw, field.Type.ArrayDepth)
	}

	switch field.Type.TypeOf {
	case domain.TypeOfBase, domain.TypeOfEnum:
		return field.ConvFunc(raw), nil
	case domain.TypeOfStruct:
		// 外部 proto message：走反射驱动的赋值路径
		if field.IsExternal {
			return parseExternalMessage(file, sheet, field.Name, field.Type.Name, rowIndex, raw)
		}
		st := manager.GetStruct(field.Type.Name)
		if st == nil {
			return nil, errs.New(file, sheet, field.Name, rowIndex, "结构体类型不存在", "找不到结构体类型: %s", field.Type.Name)
		}
		return parseStructMessage(st, rowIndex, raw)
	}
	return nil, nil
}

func parseStructMessage(f *base.Struct, rowIndex int, raw string) (*dynamic.Message, error) {
	item := manager.NewProto(f.FileName, f.Name)
	if item == nil {
		return nil, uerror.New(1, -1, "new %s is nil", f.Name)
	}

	// 结构体成员按 ',' 分隔（叶子层；与数组 '|' 和 map K:V ':' 正交，无歧义）
	strs := strings.Split(raw, ",")
	if len(strs) > len(f.FieldList) {
		return nil, errs.New(f.FileName, f.Sheet, "", rowIndex, "结构体字段数不匹配",
			"结构体=%s 传入元素数=%d 超过定义字段数=%d, 原始值=%q", f.Name, len(strs), len(f.FieldList), raw)
	}

	for i, field := range f.FieldList {
		if i >= len(strs) {
			break
		}
		cell := strs[i]
		if strings.TrimSpace(cell) == "" {
			logx.Warnf("[%s/%s] 结构体=%s 字段=%s 行=%d 值为空(结构体内) 跳过\n", f.FileName, f.Sheet, f.Name, field.Name, rowIndex)
			continue
		}
		if err := assignFieldValue(item, f.FileName, f.Sheet, f.Name, field, rowIndex, cell); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// parseExternalMessage 解析引用外部 proto message 的字段值。
// 与 parseStructMessage 不同，外部 message 的字段布局不在 structMgr 中，
// 而是从 proto descriptor 反射获取（GetFields），按定义顺序从 raw 的 ',' 分隔项取值。
// 每个字段按 proto 类型（标量/enum/message/repeated）转换：
//   - 标量/enum：走 convertMgr 转换（enum 视为 int32）
//   - message：递归 parseExternalMessage（嵌套结构用 ',' 不便表达，本期按需）
//   - repeated：按 '|' 分隔后逐元素转换
//
// fieldName 是 xlsx 字段名（用于错误定位），msgShortName 是外部 message 短名（如 Reward）。
func parseExternalMessage(file, sheet, fieldName, msgShortName string, rowIndex int, raw string) (*dynamic.Message, error) {
	item := manager.NewProto("", msgShortName)
	if item == nil {
		return nil, errs.New(file, sheet, fieldName, rowIndex, "外部类型构造失败",
			"无法构造外部 message: %s（请确认 -proto-src 已加载且 proto 已解析）", msgShortName)
	}

	fields := item.GetMessageDescriptor().GetFields()
	strs := strings.Split(raw, ",")
	if len(strs) > len(fields) {
		return nil, errs.New(file, sheet, fieldName, rowIndex, "外部结构体字段数不匹配",
			"message=%s 定义字段数=%d 传入元素数=%d, 原始值=%q", msgShortName, len(fields), len(strs), raw)
	}

	for i, fd := range fields {
		if i >= len(strs) {
			break
		}
		cell := strs[i]
		if strings.TrimSpace(cell) == "" {
			continue
		}
		if err := assignExternalField(item, fd, cell, file, sheet, fieldName, rowIndex); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// externalRepeatedDelim 是外部 proto message 内部 repeated 字段的元素分隔符。
// 用 '/' 而非 '|'：因为外部 message 可作为单值/[]数组/map value 出现，
// 其宿主容器的分隔符已占用 ','(成员) '|'(1维) ';'(2维/map元素) '^'(3维) ':'(K:V)，
// '/' 是唯一不与任何外层冲突的安全符号。
const externalRepeatedDelim = "/"

// assignExternalField 按单个 proto 字段描述符，把单元格字符串值赋给 dynamic.Message。
func assignExternalField(msg *dynamic.Message, fd *desc.FieldDescriptor, cell, file, sheet, fieldName string, rowIndex int) error {
	// repeated 字段（非 map）：按 externalRepeatedDelim 分隔后逐元素追加
	if fd.IsRepeated() && !fd.IsMap() {
		parts := strings.Split(cell, externalRepeatedDelim)
		values := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			if strings.TrimSpace(p) == "" {
				continue
			}
			v, err := convertExternalScalar(msg, fd, p, file, sheet, fieldName, rowIndex)
			if err != nil {
				return err
			}
			values = append(values, v)
		}
		if len(values) == 0 {
			return nil
		}
		return safeSetRepeatedFieldByName(msg, fd.GetName(), values, file, sheet, fd.GetName(), fd.GetName(), rowIndex)
	}

	v, err := convertExternalScalar(msg, fd, cell, file, sheet, fieldName, rowIndex)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	return safeSetField(msg, fd.GetName(), v, file, sheet, fd.GetName(), fd.GetName(), rowIndex)
}

// convertExternalScalar 把单元格字符串按 proto 字段类型转换为 Go 值。
// 标量/enum 走 convertMgr（enum 视为 int32），message 字段递归 parseExternalMessage。
func convertExternalScalar(msg *dynamic.Message, fd *desc.FieldDescriptor, cell, file, sheet, fieldName string, rowIndex int) (interface{}, error) {
	// message 字段：递归解析
	if mt := fd.GetMessageType(); mt != nil {
		return parseExternalMessage(file, sheet, fd.GetName(), mt.GetName(), rowIndex, cell)
	}
	// enum 字段：按短名查枚举转换，否则按 int32
	if ed := fd.GetEnumType(); ed != nil {
		if convFunc := manager.GetConvFunc(ed.GetName()); convFunc != nil {
			return convFunc(cell), nil
		}
		return manager.GetConvFunc("int32")(cell), nil
	}
	// 标量：用 proto 类型名查 convertMgr
	convName := protoTypeToConvName(fd.GetType().String())
	convFunc := manager.GetConvFunc(convName)
	if convFunc == nil {
		return nil, errs.New(file, sheet, fieldName, rowIndex, "外部标量类型不支持",
			"字段=%s proto类型=%s", fd.GetName(), fd.GetType().String())
	}
	return convFunc(cell), nil
}

// protoTypeToConvName 把 proto 类型字符串（fd.GetType().String()，如 "TYPE_INT32"）
// 映射到 convertMgr 的标量名。
func protoTypeToConvName(typeStr string) string {
	switch typeStr {
	case "TYPE_INT32", "TYPE_SINT32", "TYPE_SFIXED32":
		return "int32"
	case "TYPE_INT64", "TYPE_SINT64", "TYPE_SFIXED64":
		return "int64"
	case "TYPE_UINT32", "TYPE_FIXED32":
		return "uint32"
	case "TYPE_UINT64", "TYPE_FIXED64":
		return "uint64"
	case "TYPE_FLOAT":
		return "float"
	case "TYPE_DOUBLE":
		return "double"
	case "TYPE_BOOL":
		return "bool"
	case "TYPE_STRING":
		return "string"
	default:
		return "string"
	}
}

// 分隔符约定（类型无关）：
//   - 元素之间统一用 mapFieldDelim(';')，无论 value 是标量还是结构体
//   - 每个 K:V 用 ':' 分隔，按首个 ':' 切分（SplitN 2）
//     因结构体成员已改用 ','、数组用 '|'，value 内部不含 ':'，故 K/V 切分天然无歧义
func parseMapValue(file, sheet string, field *base.Field, rowIndex int, raw string) (interface{}, error) {
	keyType := field.Type.KeyType
	if keyType == nil {
		return nil, errs.New(file, sheet, field.Name, rowIndex, "类型错误", "map 字段缺少 KeyType 定义")
	}
	keyConv := manager.GetConvFunc(keyType.Name)
	if keyConv == nil {
		return nil, errs.New(file, sheet, field.Name, rowIndex, "类型错误", "map key 转换函数缺失: %s", keyType.Name)
	}

	parts := strings.Split(raw, mapFieldDelim)
	result := make(map[interface{}]interface{}, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		// 只在第一个 ':' 切 K/V；value 内部（结构体 ',' / 数组 '|'）不含 ':'，无歧义
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, errs.New(file, sheet, field.Name, rowIndex, "map 元素格式错误",
				"缺少 K:V 分隔符 ':', 原始片段=%q", part)
		}
		keyStr := strings.TrimSpace(kv[0])
		valStr := kv[1]
		if keyStr == "" {
			return nil, errs.New(file, sheet, field.Name, rowIndex, "map 元素格式错误",
				"map key 为空, 原始片段=%q", part)
		}
		key := keyConv(keyStr)

		// 解析 value
		var val interface{}
		var err error
		switch field.Type.TypeOf {
		case domain.TypeOfBase, domain.TypeOfEnum:
			val = field.ConvFunc(valStr)
		case domain.TypeOfStruct:
			// 外部 proto message 作为 map value：走反射驱动赋值
			if field.IsExternal {
				val, err = parseExternalMessage(file, sheet, field.Name, field.Type.Name, rowIndex, valStr)
				if err != nil {
					return nil, err
				}
				break
			}
			st := manager.GetStruct(field.Type.Name)
			if st == nil {
				return nil, errs.New(file, sheet, field.Name, rowIndex, "结构体类型不存在",
					"找不到结构体类型: %s", field.Type.Name)
			}
			val, err = parseStructMessage(st, rowIndex, valStr)
			if err != nil {
				return nil, err
			}
		default:
			return nil, errs.New(file, sheet, field.Name, rowIndex, "类型错误",
				"map value 类型不支持: TypeOf=%d", field.Type.TypeOf)
		}
		result[key] = val
	}
	return result, nil
}

func parseArrayValue(file, sheet string, field *base.Field, rowIndex int, raw string, depth int) ([]interface{}, error) {
	delimiter, err := arrayDelimiter(depth)
	if err != nil {
		return nil, errs.New(file, sheet, field.Name, rowIndex, "类型错误", "%s", err.Error())
	}

	parts := strings.Split(raw, delimiter)
	values := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if depth == 1 {
			value, err := parseArrayElement(file, sheet, field, rowIndex, part)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			continue
		}

		nested, err := parseArrayValue(file, sheet, field, rowIndex, part, depth-1)
		if err != nil {
			return nil, err
		}

		wrapperName := base.ArrayWrapperName(file, field.Type.Name, depth-1)
		wrapper := manager.NewProto(file, wrapperName)
		if wrapper == nil {
			return nil, errs.New(file, sheet, field.Name, rowIndex, "NewProto", "new %s is nil", wrapperName)
		}
		if err := safeSetRepeatedFieldByName(wrapper, "Values", nested, file, sheet, wrapperName, "Values", rowIndex); err != nil {
			return nil, err
		}
		values = append(values, wrapper)
	}
	return values, nil
}

func parseArrayElement(file, sheet string, field *base.Field, rowIndex int, raw string) (interface{}, error) {
	switch field.Type.TypeOf {
	case domain.TypeOfBase, domain.TypeOfEnum:
		return field.ConvFunc(raw), nil
	case domain.TypeOfStruct:
		// 外部 proto message 数组元素：走反射驱动赋值
		if field.IsExternal {
			return parseExternalMessage(file, sheet, field.Name, field.Type.Name, rowIndex, raw)
		}
		st := manager.GetStruct(field.Type.Name)
		if st == nil {
			return nil, errs.New(file, sheet, field.Name, rowIndex, "结构体类型不存在", "找不到结构体类型: %s", field.Type.Name)
		}
		return parseStructMessage(st, rowIndex, raw)
	}
	return nil, nil
}

// arrayDelimiter 返回第 depth 维数组的元素分隔符（类型无关）。
// 标量与结构体数组共用同一张层级表，不再按 typeOf 区分。
func arrayDelimiter(depth int) (string, error) {
	if depth <= 0 || depth > domain.MaxArrayDepth {
		return "", fmt.Errorf("数组最大仅支持到%d维", domain.MaxArrayDepth)
	}
	return arrayDelimiters[depth-1], nil
}

// 安全包装，捕获 dynamic.Message SetFieldByName 的 panic，转为可读错误
func safeSetField(msg *dynamic.Message, fieldName string, value interface{}, file, sheet, msgName, cfgField string, row int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(file, sheet, cfgField, row, "赋值错误", "Message=%s Field=%s ValueType=%T panic=%v", msgName, fieldName, value, r)
		}
	}()
	msg.SetFieldByName(fieldName, value)
	return nil
}

// safeSetMapField 将解析得到的 map[interface{}]interface{} 赋给 proto3 map 字段。
// jhump/protoreflect 的 dynamic.Message 对 map 字段：
//   - 用 field descriptor 的 TryMapKey/TryPut 按条目写入最稳，避开 SetField 对 map 具体类型的要求
func safeSetMapField(msg *dynamic.Message, fieldName string, value interface{}, file, sheet, msgName, cfgField string, row int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(file, sheet, cfgField, row, "赋值错误", "Message=%s Field=%s MapType=%T panic=%v", msgName, fieldName, value, r)
		}
	}()

	desc := msg.GetMessageDescriptor().FindFieldByName(fieldName)
	if desc == nil {
		return errs.New(file, sheet, cfgField, row, "赋值错误", "Message=%s 找不到 map 字段=%s", msgName, fieldName)
	}
	if !desc.IsMap() {
		return errs.New(file, sheet, cfgField, row, "赋值错误", "Message=%s Field=%s 不是 map 类型字段", msgName, fieldName)
	}

	entries, ok := value.(map[interface{}]interface{})
	if !ok {
		return errs.New(file, sheet, cfgField, row, "赋值错误", "map 字段解析结果不是 map[interface{}]interface{}: %T", value)
	}

	for k, v := range entries {
		// value 是 nil（如结构体值为空跳过）时跳过该条目
		if v == nil {
			continue
		}
		if err := msg.TryPutMapField(desc, k, v); err != nil {
			return errs.New(file, sheet, cfgField, row, "赋值错误",
				"Message=%s Field=%s key=%v valueType=%T err=%v", msgName, fieldName, k, v, err)
		}
	}
	return nil
}

func safeSetRepeatedFieldByName(msg *dynamic.Message, fieldName string, values []interface{}, file, sheet, msgName, cfgField string, row int) error {
	for _, value := range values {
		if err := safeAddRepeatedFieldByName(msg, fieldName, value, file, sheet, msgName, cfgField, row); err != nil {
			return err
		}
	}
	return nil
}

// 安全包装，捕获 AddRepeatedFieldByName 的 panic
func safeAddRepeatedFieldByName(msg *dynamic.Message, fieldName string, value interface{}, file, sheet, msgName, cfgField string, row int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.New(file, sheet, cfgField, row, "追加错误", "Message=%s Field=%s ValueType=%T panic=%v", msgName, fieldName, value, r)
		}
	}()
	msg.AddRepeatedFieldByName(fieldName, value)
	return nil
}

// buildMapKey 根据配置的主键字段名列表，从 meta map 中拼接 map key
func buildMapKey(meta map[string]interface{}, keyFields []string) string {
	if len(keyFields) == 1 {
		return fmt.Sprintf("%v", meta[keyFields[0]])
	}
	parts := make([]string, 0, len(keyFields))
	for _, name := range keyFields {
		parts = append(parts, fmt.Sprintf("%v", meta[name]))
	}
	return strings.Join(parts, "_")
}

func dynamicValueToInterface(val interface{}) interface{} {
	switch v := val.(type) {
	case *dynamic.Message:
		// proto3 message 字段未设置时 GetField 可能返回类型化的 nil（*dynamic.Message(nil)）
		if v == nil {
			return nil
		}
		if isArrayWrapperMessage(v) {
			for _, field := range v.GetKnownFields() {
				if field.GetName() == "Values" {
					return dynamicValueToInterface(v.GetField(field))
				}
			}
			return []interface{}{}
		}
		result := make(map[string]interface{})
		for _, field := range v.GetKnownFields() {
			// proto3 map 字段在 dynamic 里 GetField 返回 map[interface{}]interface{}
			if field.IsMap() {
				result[field.GetName()] = mapFieldToInterface(v.GetField(field))
				continue
			}
			result[field.GetName()] = dynamicValueToInterface(v.GetField(field))
		}
		return result
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = dynamicValueToInterface(item)
		}
		return out
	default:
		return val
	}
}

// mapFieldToInterface 把 proto3 map 字段（GetField 返回 map[interface{}]interface{}）
// 转为 JSON 友好的 map[string]interface{}，key 统一 fmt 成字符串。
func mapFieldToInterface(val interface{}) interface{} {
	m, ok := val.(map[interface{}]interface{})
	if !ok {
		return val
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[fmt.Sprintf("%v", k)] = dynamicValueToInterface(v)
	}
	return out
}

func isArrayWrapperMessage(msg *dynamic.Message) bool {
	desc := msg.GetMessageDescriptor()
	return desc != nil && strings.HasPrefix(desc.GetName(), "PBARR_")
}
