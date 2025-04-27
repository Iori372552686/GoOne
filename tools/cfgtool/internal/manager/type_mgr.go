package manager

import (
	"sort"
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

var (
	configMgr = make(map[string]*base.Config)
	structMgr = make(map[string]*base.Struct)
	enumMgr   = make(map[string]*base.Enum)
)

func GetIndexMap() (rets []int) {
	tmps := map[int]struct{}{}
	for _, item := range configMgr {
		for _, st := range item.IndexList {
			count := len(st.List)
			if _, ok := tmps[count]; count > 1 && !ok {
				rets = append(rets, count)
			}
			tmps[len(st.List)] = struct{}{}
		}
	}
	return
}

// -----config-------
func GetOrNewConfig(file, sheet, name string) *base.Config {
	if val, ok := configMgr[name]; ok {
		return val
	}
	configMgr[name] = &base.Config{
		Name:     name,
		FileName: file,
		Sheet:    sheet,
		Fields:   make(map[string]*base.Field),
		Indexs:   make(map[int][]*base.Index),
	}
	return configMgr[name]
}

func GetConfigList() (rets []*base.Config) {
	for _, val := range configMgr {
		rets = append(rets, val)
	}
	sort.Slice(rets, func(i, j int) bool {
		return strings.Compare(rets[i].Name, rets[j].Name) <= 0
	})
	return
}

func GetConfigMap() map[string]*base.Config {
	return configMgr
}

func GetConfig(name string) *base.Config {
	if val, ok := configMgr[name]; ok {
		return val
	}
	return nil
}

// -----struct-------
func GetOrNewStruct(file, sheet, name string) *base.Struct {
	if val, ok := structMgr[name]; ok {
		return val
	}
	structMgr[name] = &base.Struct{
		Name:     name,
		Sheet:    sheet,
		FileName: file,
		Fields:   make(map[string]*base.Field),
		Converts: make(map[string][]*base.Field),
	}
	return structMgr[name]
}

func GetStructList() (rets []*base.Struct) {
	for _, val := range structMgr {
		rets = append(rets, val)
	}
	sort.Slice(rets, func(i, j int) bool {
		return strings.Compare(rets[i].Name, rets[j].Name) <= 0
	})
	return
}

func GetStruct(name string) *base.Struct {
	if val, ok := structMgr[name]; ok {
		return val
	}
	return nil
}

func GetStructMap() map[string]*base.Struct {
	return structMgr
}

// -----enum-------
func GetOrNewEnum(name string) *base.Enum {
	if val, ok := enumMgr[name]; ok {
		return val
	}
	enumMgr[name] = &base.Enum{
		Name:   name,
		Values: make(map[string]*base.EValue),
	}
	return enumMgr[name]
}

func GetEnum(name string) *base.Enum {
	return enumMgr[name]
}

func GetEnumMap() map[string]*base.Enum {
	return enumMgr
}

func GetEnumList() (rets []*base.Enum) {
	for _, val := range enumMgr {
		rets = append(rets, val)
	}
	sort.Slice(rets, func(i, j int) bool {
		return strings.Compare(rets[i].Name, rets[j].Name) <= 0
	})
	return
}
