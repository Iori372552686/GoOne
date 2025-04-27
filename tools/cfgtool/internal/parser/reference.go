package parser

import (
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
)

func parseReference() {
	for _, item := range manager.GetStructMap() {
		tmps := map[string]struct{}{}
		for _, field := range item.FieldList {
			switch field.Type.TypeOf {
			case domain.TypeOfEnum:
				en := manager.GetEnum(field.Type.Name)
				if en.FileName != item.FileName {
					tmps[en.FileName] = struct{}{}
				}
			case domain.TypeOfStruct:
				st := manager.GetStruct(field.Type.Name)
				if st.FileName != item.FileName {
					tmps[st.FileName] = struct{}{}
				}
			case domain.TypeOfConfig:
				cfg := manager.GetConfig(field.Type.Name)
				if cfg.FileName != item.FileName {
					tmps[cfg.FileName] = struct{}{}
				}
			}
		}
		manager.AddRef(item.FileName, tmps)
	}

	for _, item := range manager.GetConfigMap() {
		tmps := map[string]struct{}{}
		for _, field := range item.FieldList {
			switch field.Type.TypeOf {
			case domain.TypeOfEnum:
				en := manager.GetEnum(field.Type.Name)
				if en.FileName != item.FileName {
					tmps[en.FileName] = struct{}{}
				}
			case domain.TypeOfStruct:
				st := manager.GetStruct(field.Type.Name)
				if st.FileName != item.FileName {
					tmps[st.FileName] = struct{}{}
				}
			case domain.TypeOfConfig:
				cfg := manager.GetConfig(field.Type.Name)
				if cfg.FileName != item.FileName {
					tmps[cfg.FileName] = struct{}{}
				}
			}
		}
		manager.AddRef(item.FileName, tmps)
	}
}
