package service

import (
	"strings"

	"github.com/Iori372552686/GoOne/lib/uerror"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
)

func GenData() error {
	for _, cfg := range manager.GetConfigMap() {
		// 反射new一个对象
		ary := manager.NewProto(cfg.FileName, cfg.Name+"Ary")
		if ary == nil {
			return uerror.New(1, -1, "new %sAry is nil", cfg.Name)
		}
		// 加载xlsx数据
		tab := manager.GetTable(cfg.FileName, cfg.Sheet)
		for _, vals := range tab.Rows[3:] {
			item, err := configValue(cfg, vals...)
			if err != nil {
				return err
			}
			ary.AddRepeatedFieldByName("Ary", item)
		}
		// 保存数据
		if len(domain.JsonPath) > 0 {
			buf, err := ary.MarshalJSONIndent()
			if err != nil {
				return err
			}
			if err := base.Save(domain.JsonPath, cfg.Name+".json", buf); err != nil {
				return err
			}
		}
		if len(domain.BytesPath) > 0 {
			buf, err := ary.Marshal()
			if err != nil {
				return err
			}
			if err := base.Save(domain.BytesPath, cfg.Name+".bytes", buf); err != nil {
				return err
			}
		}
		if len(domain.TextPath) > 0 {
			buf, err := ary.MarshalTextIndent()
			if err != nil {
				return err
			}
			if err := base.Save(domain.TextPath, cfg.Name+".conf", buf); err != nil {
				return err
			}
		}
	}
	manager.Clear()
	return nil
}

func configValue(f *base.Config, vals ...string) (interface{}, error) {
	// 反射new一个对象
	item := manager.NewProto(f.FileName, f.Name)
	if item == nil {
		return nil, uerror.New(1, -1, "new %s is nil", f.Name)
	}

	for i, field := range f.FieldList {
		if field.Position >= len(vals) {
			break
		}

		switch field.Type.TypeOf {
		case domain.TypeOfBase, domain.TypeOfEnum:
			item.SetFieldByName(field.Name, fieldValue(field, vals[i]))

		case domain.TypeOfStruct:
			st := manager.GetStruct(field.Type.Name)
			rets, err := structValue(st, strings.Split(vals[i], "|")...)
			if err != nil {
				return nil, err
			}

			switch field.Type.ValueOf {
			case domain.ValueOfBase:
				if len(rets) > 0 {
					item.SetFieldByName(field.Name, rets[0])
				}
			case domain.ValueOfList:
				item.SetFieldByName(field.Name, rets)
			}
		}
	}
	return item, nil
}

func structValue(f *base.Struct, vals ...string) (rets []interface{}, err error) {
	for _, val := range vals {
		item := manager.NewProto(f.FileName, f.Name)
		if item == nil {
			return nil, uerror.New(1, -1, "new %s is nil", f.Name)
		}

		strs := strings.Split(val, ":")
		if len(strs) < len(f.Converts[strs[0]]) {
			return nil, uerror.New(1, -1, "%s:%s配置错误: %s", f.FileName, f.Sheet, val)
		}

		for i, field := range f.Converts[strs[0]] {
			item.SetFieldByName(field.Name, fieldValue(field, strs[i]))
		}
		rets = append(rets, item)
	}
	return
}

func fieldValue(f *base.Field, val string) interface{} {
	switch f.Type.ValueOf {
	case domain.ValueOfBase:
		return f.ConvFunc(val)
	case domain.ValueOfList:
		return f.Convert(strings.Split(val, ",")...)
	}
	return nil
}
