package service

import (
	"bytes"
	"sort"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/templ"
)

type ProtoInfo struct {
	GoPackage     string // option go_package 值，由 domain.PbPath 推导
	RefList       []string
	EnumList      []*base.Enum
	StructList    []*base.Struct
	ConfigList    []*base.Config
	ArrayWrappers []*ProtoArrayWrapper
}

type ProtoArrayWrapper struct {
	Name      string
	ValueType string
	Level     int
}

func collectArrayWrappers(fileName string, data *ProtoInfo) []*ProtoArrayWrapper {
	items := map[string]*ProtoArrayWrapper{}
	collect := func(fields []*base.Field) {
		for _, field := range fields {
			if field.Type.ArrayDepth <= 1 {
				continue
			}
			for level := 1; level < field.Type.ArrayDepth; level++ {
				name := base.ArrayWrapperName(fileName, field.Type.Name, level)
				valueType := field.Type.Name
				if level > 1 {
					valueType = base.ArrayWrapperName(fileName, field.Type.Name, level-1)
				}
				items[name] = &ProtoArrayWrapper{
					Name:      name,
					ValueType: valueType,
					Level:     level,
				}
			}
		}
	}

	for _, item := range data.StructList {
		collect(item.FieldList)
	}
	for _, item := range data.ConfigList {
		collect(item.FieldList)
	}

	wrappers := make([]*ProtoArrayWrapper, 0, len(items))
	for _, item := range items {
		wrappers = append(wrappers, item)
	}
	sort.Slice(wrappers, func(i, j int) bool {
		if wrappers[i].Level != wrappers[j].Level {
			return wrappers[i].Level < wrappers[j].Level
		}
		return wrappers[i].Name < wrappers[j].Name
	})
	return wrappers
}

func GenProto() error {
	// 根据文件分类
	tmps := map[string]*ProtoInfo{}
	for _, val := range manager.GetEnumList() {
		sort.Slice(val.ValueList, func(i, j int) bool {
			return val.ValueList[i].Value < val.ValueList[j].Value
		})
		if _, ok := tmps[val.FileName]; !ok {
			tmps[val.FileName] = &ProtoInfo{}
		}
		tmps[val.FileName].EnumList = append(tmps[val.FileName].EnumList, val)
	}

	for _, val := range manager.GetStructList() {
		if _, ok := tmps[val.FileName]; !ok {
			tmps[val.FileName] = &ProtoInfo{}
		}
		tmps[val.FileName].StructList = append(tmps[val.FileName].StructList, val)
	}

	for _, val := range manager.GetConfigList() {
		if _, ok := tmps[val.FileName]; !ok {
			tmps[val.FileName] = &ProtoInfo{}
		}
		tmps[val.FileName].ConfigList = append(tmps[val.FileName].ConfigList, val)
	}

	// 生成proto文件
	buf := bytes.NewBuffer(nil)
	for fileName, data := range tmps {
		buf.Reset()
		data.RefList = manager.GetRefList(fileName)
		data.ArrayWrappers = collectArrayWrappers(fileName, data)
		// go_package：与 -pb 指向的包路径一致，便于 protoc --go_opt=module 剥前缀后
		// 落到 protocol/ 目录；分号后是 Go package 名（与 core proto 一致用 g1_protocol）。
		// 例如 -pb=github.com/Iori372552686/g1_common/protocol
		//     -> option go_package = "github.com/Iori372552686/g1_common/protocol;g1_protocol";
		data.GoPackage = domain.PbPath + ";g1_protocol"
		if err := templ.ProtoTpl.Execute(buf, data); err != nil {
			return errs.Wrap(err, fileName, "", "", 0, "生成错误", "生成proto文件内容失败")
		}
		manager.AddProto(fileName, buf)
	}
	return nil
}

func SaveProto() error {
	if len(domain.ProtoPath) <= 0 {
		return nil
	}

	for fileName, data := range manager.GetProtoMap() {
		// 外部 proto（-proto-src 载入）只参与类型解析，不输出到 -proto 目录；
		// 否则会把 core/service/storage proto 复制进配置 proto 目录，
		// 后续 protoc 编译时会与 core/ 原文件重复定义而失败。
		if manager.IsExternalProto(fileName) {
			continue
		}
		if err := base.Save(domain.ProtoPath, fileName, []byte(data)); err != nil {
			return errs.Wrap(err, fileName, "", "", 0, "保存错误", "保存proto失败")
		}
	}
	return nil
}
