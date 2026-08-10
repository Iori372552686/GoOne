package service

import (
	"bytes"
	"path"

	"strings"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/errs"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/logx"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/templ"
	"github.com/iancoleman/strcase"
)

type ConfigInfo struct {
	PbPkg  string
	Pkg    string
	Prefix string // 方法/类型前缀（Config 类型名去掉 "Config" 后缀），避免同包多 config 命名冲突
	*base.Config
}

type IndexInfo struct {
	Pkg       string
	IndexList []int
}

func GenCode() error {
	buf := bytes.NewBuffer(nil)

	if len(domain.PbPath) <= 0 || len(domain.CodePath) <= 0 || len(domain.Module) <= 0 {

		return nil
	}

	// 生成索引
	if err := genIndex(buf); err != nil {
		return errs.Wrap(err, "", "", "", 0, "生成错误", "生成索引失败")
	}
	// 对文件分类
	for _, st := range manager.GetConfigMap() {
		buf.Reset()
		// 目录/package 用功能名（snake）；文件名用 gdata_<feature>_<table>.go。
		// 缺省（feature/table 为空，如旧式无 @ 命名）退化为基于 Name 的旧行为，保持兼容。
		feature := st.Feature
		table := st.Table
		pkgName := base.GoPkgName(feature)
		fileName := base.GoFileName(feature, table)
		if feature == "" || table == "" {
			// 兼容路径：未走 @ 命名体系时，沿用 Name 推导
			dataName := strings.TrimSuffix(st.Name, "Config")
			pkgName = strcase.ToSnake(st.Name)
			fileName = dataName + "Data.gen.go"
		}
		item := &ConfigInfo{
			PbPkg:  domain.PkgName,
			Pkg:    pkgName,
			Prefix: strings.TrimSuffix(st.Name, "Config"),
			Config: st,
		}
		if err := templ.CodeTpl.Execute(buf, item); err != nil {
			return errs.Wrap(err, st.FileName, st.Sheet, "", 0, "生成错误", "渲染代码模板失败")
		}
		// 保存代码
		if err := base.SaveGo(path.Join(domain.CodePath, pkgName), fileName, buf.Bytes()); err != nil {
			return errs.Wrap(err, st.FileName, st.Sheet, "", 0, "保存错误", "保存代码失败")
		}
	}

	logx.Successf("Go代码生成完成")
	return nil
}
