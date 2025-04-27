package base

import (
	"fmt"
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/spf13/cast"
)

// E|道具类型-金币|PropertyType|Coin|1
func (d *Enum) AddValue(strs ...string) {
	val := &EValue{
		Name:  strs[2] + "_" + strs[3],
		Desc:  strs[1],
		Value: cast.ToInt32(strs[4]),
	}
	d.ValueList = append(d.ValueList, val)
	d.Values[val.Desc] = val
}

func (d *Struct) AddField(f *Field) {
	d.Fields[f.Name] = f
	d.FieldList = append(d.FieldList, f)
}

func (d *Config) AddField(f *Field) {
	d.Fields[f.Name] = f
	d.FieldList = append(d.FieldList, f)
}

func (d *Config) AddIndex(ind *Index) {
	d.Indexs[ind.Type.ValueOf] = append(d.Indexs[ind.Type.ValueOf], ind)
	d.IndexList = append(d.IndexList, ind)
}

// 成员函数参数
func (d *Index) Arg(split string) string {
	strs := []string{}
	for _, val := range d.List {
		strs = append(strs, val.Name+" "+val.Type.GetType())
	}
	return strings.Join(strs, split)
}

func (d *Index) Value(ref, split string) string {
	strs := []string{}
	for _, val := range d.List {
		if len(ref) > 0 {
			strs = append(strs, ref+"."+val.Name)
		} else {
			strs = append(strs, val.Name)
		}
	}
	return strings.Join(strs, split)
}

// 获取类型字符串
func (d *Type) GetType() string {
	switch d.TypeOf {
	case domain.TypeOfBase:
		switch d.ValueOf {
		case domain.ValueOfBase:
			return d.Name
		case domain.ValueOfList:
			return fmt.Sprintf("[]%s", d.Name)
		}
	case domain.TypeOfEnum:
		switch d.ValueOf {
		case domain.ValueOfBase:
			if len(domain.PkgName) > 0 {
				return domain.PkgName + "." + d.Name
			}
			return d.Name
		case domain.ValueOfList:
			if len(domain.PkgName) > 0 {
				return fmt.Sprintf("[]%s.%s", domain.PkgName, d.Name)
			}
			return fmt.Sprintf("[]%s", d.Name)
		}
	case domain.TypeOfStruct, domain.TypeOfConfig:
		switch d.ValueOf {
		case domain.ValueOfBase:
			if len(domain.PkgName) > 0 {
				return fmt.Sprintf("*%s.%s", domain.PkgName, d.Name)
			}
			return fmt.Sprintf("*%s", d.Name)
		case domain.ValueOfList:
			if len(domain.PkgName) > 0 {
				return fmt.Sprintf("[]*%s.%s", domain.PkgName, d.Name)
			}
			return fmt.Sprintf("[]*%s", d.Name)
		}
	}
	return ""
}

func (d *Field) Convert(vals ...string) (rets []interface{}) {
	for _, val := range vals {
		rets = append(rets, d.ConvFunc(val))
	}
	return
}

type FieldList []*Field

func (d FieldList) GetIndexName() string {
	if len(d) == 1 {
		return d[0].Type.GetType()
	}
	strs := []string{}
	for _, val := range d {
		strs = append(strs, val.Type.GetType())
	}
	if len(domain.PkgName) > 0 {
		return fmt.Sprintf("%s.Index%d[%s]", domain.PkgName, len(d), strings.Join(strs, ","))
	}
	return fmt.Sprintf("Index%d[%s]", len(d), strings.Join(strs, ","))
}
