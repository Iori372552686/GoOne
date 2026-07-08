package manager

import (
	"errors"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
	"github.com/spf13/cast"
	"strings"
)

var (
	convertMgr = make(map[string]*base.Convert)
)

// 选项配置
type ExtractOptions struct {
	AllowDecimal      bool // 是否允许小数点
	AllowLeadingZeros bool // 是否保留前导零
}

// 从字符串中提取数字部分（包含扩展选项）
func extractDigits(s string, opts ...*ExtractOptions) (string, error) {
	if len(s) == 0 {
		return "", nil
	}

	opt := &ExtractOptions{}
	if len(opts) > 0 {
		opt = opts[0]
	}

	var result strings.Builder
	expectDigit := false
	decimalPointAdded := false

	for i, r := range s {
		switch {
		case r == '-' && result.Len() == 0:
			// 开头允许负号
			result.WriteRune(r)
			expectDigit = true

		case r == '.' && opt.AllowDecimal && !decimalPointAdded:
			result.WriteRune(r)
			expectDigit = true
			decimalPointAdded = true

		case r == '0' && result.Len() > 0 && result.String() == "-":
			// 负号后的第一个零允许
			result.WriteRune(r)
			expectDigit = false

		case r >= '1' && r <= '9':
			result.WriteRune(r)
			expectDigit = false

		case r == '0' && (!opt.AllowLeadingZeros || result.Len() > 0 || i > 0):
			// 只有允许前导零或非起始位置时才记录零
			result.WriteRune(r)
			expectDigit = false
		}
	}

	// 处理只有负号没有数字的情况
	if expectDigit {
		return "", errors.New("单元格无效格式: 负号后无数字")
	}

	final := result.String()

	// 检查有效结果
	if len(final) == 0 {
		return "", errors.New("单元格无数字提取")
	}

	return final, nil
}

func GetConvFunc(name string) func(string) interface{} {
	if val, ok := convertMgr[name]; ok {
		return val.ConvFunc
	}

	// 默认枚举转换函数
	if item, ok := enumMgr[name]; ok {
		return func(str string) interface{} {
			if vv, ok := item.Values[str]; ok {
				return vv.Value
			}
			return cast.ToInt32(str)
		}
	}
	return nil
}

func GetConvType(name string) string {
	if val, ok := convertMgr[name]; ok {
		return val.Name
	}
	return name
}

func convertInt32(str string) interface{} {
	if str == "" {
		return int32(0)
	}

	str, err := extractDigits(str)
	if err != nil {
		logx.Warnf("%s", err.Error())
	}
	return cast.ToInt32(str)
}

func convertInt64(str string) interface{} {
	if str == "" {
		return int64(0)
	}

	str, err := extractDigits(str)
	if err != nil {
		logx.Warnf("%s", err.Error())
	}
	return cast.ToInt64(str) //str如有空格 cast.ToInt64会返回为0
}

func convertUint32(str string) interface{} {
	if str == "" {
		return int32(0)
	}

	str, err := extractDigits(str)
	if err != nil {
		logx.Warnf("%s", err.Error())
	}
	return cast.ToUint32(str)
}

func convertUint64(str string) interface{} {
	if str == "" {
		return int64(0)
	}

	str, err := extractDigits(str)
	if err != nil {
		logx.Warnf("%s", err.Error())
	}
	return cast.ToUint64(str)
}

func convertFloat64(str string) interface{} {
	if str == "" {
		return float64(0)
	}
	return cast.ToFloat64(str)
}

func init() {
	convertMgr["int"] = &base.Convert{
		Name:     "int32",
		ConvFunc: convertInt32,
	}
	convertMgr["int8"] = convertMgr["int"]
	convertMgr["int16"] = convertMgr["int"]
	convertMgr["int32"] = convertMgr["int"]
	convertMgr["int64"] = &base.Convert{
		Name:     "int64",
		ConvFunc: convertInt64,
	}

	convertMgr["uint"] = &base.Convert{
		Name:     "uint32",
		ConvFunc: convertUint32,
	}
	convertMgr["uint8"] = convertMgr["uint"]
	convertMgr["uint16"] = convertMgr["uint"]
	convertMgr["uint32"] = convertMgr["uint"]
	convertMgr["uint64"] = &base.Convert{
		Name:     "uint64",
		ConvFunc: convertUint64,
	}

	convertMgr["[]int"] = &base.Convert{
		Name:     "Int64Array",
		ConvFunc: convertInt32,
	}
	convertMgr["[]int8"] = convertMgr["[]int"]
	convertMgr["[]int16"] = convertMgr["[]int"]
	convertMgr["[]int32"] = convertMgr["[]int"]
	convertMgr["[]int64"] = &base.Convert{
		Name:     "Int64Array",
		ConvFunc: convertInt64,
	}

	convertMgr["[]uint"] = &base.Convert{
		Name:     "Int64Array",
		ConvFunc: convertUint32,
	}
	convertMgr["[]uint8"] = convertMgr["[]uint"]
	convertMgr["[]uint16"] = convertMgr["[]uint"]
	convertMgr["[]uint32"] = convertMgr["[]uint"]
	convertMgr["[]uint64"] = &base.Convert{
		Name:     "Int64Array",
		ConvFunc: convertUint64,
	}

	convertMgr["[]string"] = &base.Convert{
		Name: "StringArray",
		ConvFunc: func(str string) interface{} {
			return str
		},
	}

	convertMgr["[]double"] = &base.Convert{
		Name:     "DoubleArray",
		ConvFunc: convertFloat64,
	}
	convertMgr["[]float"] = convertMgr["[]double"]
	convertMgr["[]float32"] = convertMgr["[]double"]
	convertMgr["[]float64"] = convertMgr["[]double"]

	convertMgr["float"] = &base.Convert{
		Name: "float",
		ConvFunc: func(str string) interface{} {
			return cast.ToFloat32(str)
		},
	}

	convertMgr["float64"] = &base.Convert{
		Name: "double", // 在protobuf中double对应float64
		ConvFunc: func(str string) interface{} {
			return cast.ToFloat64(str)
		},
	}
	convertMgr["double"] = convertMgr["float64"]

	convertMgr["bool"] = &base.Convert{
		Name: "bool",
		ConvFunc: func(str string) interface{} {
			return cast.ToBool(str)
		},
	}
	convertMgr["string"] = &base.Convert{
		Name: "string",
		ConvFunc: func(str string) interface{} {
			return str
		},
	}
}
