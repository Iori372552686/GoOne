package manager

import (
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/spf13/cast"
)

var (
	convertMgr = make(map[string]*base.Convert)
)

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

func init() {
	convertMgr["int"] = &base.Convert{
		Name: "int32",
		ConvFunc: func(str string) interface{} {
			return cast.ToInt32(str)
		},
	}
	convertMgr["int8"] = convertMgr["int"]
	convertMgr["int16"] = convertMgr["int"]
	convertMgr["int32"] = convertMgr["int"]
	convertMgr["int64"] = &base.Convert{
		Name: "int64",
		ConvFunc: func(str string) interface{} {
			return cast.ToInt64(str)
		},
	}

	convertMgr["uint"] = &base.Convert{
		Name: "uint32",
		ConvFunc: func(str string) interface{} {
			return cast.ToUint32(str)
		},
	}
	convertMgr["uint8"] = convertMgr["uint"]
	convertMgr["uint16"] = convertMgr["uint"]
	convertMgr["uint32"] = convertMgr["uint"]
	convertMgr["uint64"] = &base.Convert{
		Name: "uint64",
		ConvFunc: func(str string) interface{} {
			return cast.ToUint64(str)
		},
	}

	convertMgr["float"] = &base.Convert{
		Name: "float64",
		ConvFunc: func(str string) interface{} {
			return cast.ToFloat64(str)
		},
	}
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
