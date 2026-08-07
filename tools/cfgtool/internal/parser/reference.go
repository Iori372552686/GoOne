package parser

import (
	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
)

func parseReference() {
	for _, item := range manager.GetStructMap() {
		manager.AddRef(item.FileName, collectRefs(item.FieldList, item.FileName))
	}

	for _, item := range manager.GetConfigMap() {
		manager.AddRef(item.FileName, collectRefs(item.FieldList, item.FileName))
	}
}

// collectRefs 收集字段列表里所有「跨文件类型引用」对应的 proto 文件基名。
// 返回值的约定：统一为「不含 .proto 后缀」的 import 基名，proto 模板会拼成 import "<base>.proto";
// 涵盖两类：
//  1. 内置 enum/struct/config 跨文件引用（定义文件与当前文件 currentFile 不同）
//  2. 外部 proto 类型（field.IsExternal）—— 取 externalMgr 的 ProtoFile 并剥 .proto 后缀，
//     如 "proto/core/struct"
func collectRefs(fields []*base.Field, currentFile string) map[string]struct{} {
	tmps := map[string]struct{}{}
	for _, field := range fields {
		// 外部 proto 类型：按短名查 ProtoFile，剥 .proto 后缀以适配模板
		if field.IsExternal {
			var protoFile string
			if et := manager.GetExternalMsg(field.Type.Name); et != nil {
				protoFile = et.ProtoFile
			} else if et := manager.GetExternalEnum(field.Type.Name); et != nil {
				protoFile = et.ProtoFile
			}
			if protoFile != "" {
				tmps[strings.TrimSuffix(protoFile, ".proto")] = struct{}{}
			}
			continue
		}
		// 内置跨文件引用：仅收集定义文件与当前文件不同的引用（避免自 import 循环）
		switch field.Type.TypeOf {
		case domain.TypeOfEnum:
			if en := manager.GetEnum(field.Type.Name); en != nil && en.FileName != "" && en.FileName != currentFile {
				tmps[en.FileName] = struct{}{}
			}
		case domain.TypeOfStruct:
			if st := manager.GetStruct(field.Type.Name); st != nil && st.FileName != "" && st.FileName != currentFile {
				tmps[st.FileName] = struct{}{}
			}
		case domain.TypeOfConfig:
			if cfg := manager.GetConfig(field.Type.Name); cfg != nil && cfg.FileName != "" && cfg.FileName != currentFile {
				tmps[cfg.FileName] = struct{}{}
			}
		}
	}
	return tmps
}
