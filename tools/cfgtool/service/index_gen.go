package service

import (
	"bytes"

	"github.com/Iori372552686/GoOne/lib/uerror"

	"sort"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/templ"
)

func genIndex(buf *bytes.Buffer) error {
	indexs := &IndexInfo{
		Pkg:       "g1_protocol",
		IndexList: manager.GetIndexMap(),
	}

	if len(indexs.IndexList) > 0 {
		sort.Slice(indexs.IndexList, func(i, j int) bool {
			return indexs.IndexList[i] < indexs.IndexList[j]
		})

		buf.Reset()
		if err := templ.IndexTpl.Execute(buf, indexs); err != nil {
			return uerror.New(1, -1, "gen index file error: %s", err.Error())
		}
		return base.SaveGo(domain.ProtoPath, "index.gen.go", buf.Bytes())
	}
	return nil
}
