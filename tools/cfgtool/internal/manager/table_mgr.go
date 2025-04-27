package manager

import (
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

var (
	tableMgr = make(map[string]*base.Table)
	groupMgr = make(map[int][]*base.Table)
)

func AddTable(file, sheet string, typeOf int, t string, rows [][]string, rules []string) {
	key := file + ":" + sheet
	val := &base.Table{
		Type:     t,
		TypeOf:   typeOf,
		Sheet:    sheet,
		FileName: file,
		Rules:    rules,
		Rows:     rows,
	}
	tableMgr[key] = val
	groupMgr[val.TypeOf] = append(groupMgr[val.TypeOf], val)
}

func GetTable(file, sheet string) *base.Table {
	return tableMgr[file+":"+sheet]
}

func GetTableList(typeOf int) []*base.Table {
	return groupMgr[typeOf]
}

func GetTypeOf(name string) int {
	name = GetConvType(name)
	if _, ok := enumMgr[name]; ok {
		return domain.TypeOfEnum
	}
	if _, ok := structMgr[name]; ok {
		return domain.TypeOfStruct
	}
	return domain.TypeOfBase
}

func GetValueOf(name string) int {
	if strings.HasPrefix(name, "[]") {
		return domain.ValueOfList
	}
	return domain.ValueOfBase
}
