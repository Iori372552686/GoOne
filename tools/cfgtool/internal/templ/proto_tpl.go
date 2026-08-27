package templ

import (
	"text/template"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

func protoFieldModifier(t *base.Type) string {
	// map 字段不加 repeated（proto3 map 本身是字段的类型修饰）
	if t.Container == domain.ContainerMap {
		return ""
	}
	if t.ArrayDepth > 0 || t.ValueOf == domain.ValueOfList {
		return "repeated "
	}
	return ""
}

func protoFieldType(fileName string, t *base.Type) string {
	// map<K,V>：proto3 原生 map 语法，K/V 直接用规范类型名
	if t.Container == domain.ContainerMap && t.KeyType != nil {
		return "map<" + t.KeyType.Name + ", " + t.Name + ">"
	}
	depth := t.ArrayDepth
	if depth == 0 && t.ValueOf == domain.ValueOfList {
		depth = 1
	}
	if depth <= 1 {
		return t.Name
	}
	return base.ArrayWrapperName(fileName, t.Name, depth-1)
}

const protoTpl = `
/*
* 本代码由xlsx工具生成，请勿手动修改
*/

syntax = "proto3";

package g1.protocol;

option go_package = "{{.GoPackage}}";

{{range $item := .RefList -}}
import "{{$item}}.proto";
{{end}}

{{- range $item := .EnumList}}
enum {{$item.Name}} {
	{{- range $field := $item.ValueList}}
	{{$field.Name}} = {{$field.Value}}; // {{$field.Desc}}
	{{- end}}
}
{{end}}

{{- range $item := .ArrayWrappers}}
message {{$item.Name}} {
	repeated {{$item.ValueType}} Values = 1;
}
{{end}}

{{- range $item := .StructList}}
message {{$item.Name}} {
	{{- $fileName := $item.FileName}}
	{{- range $pos, $field := $item.FieldList}}
	{{protofieldmodifier $field.Type}}{{protofieldtype $fileName $field.Type}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
{{- end}}
}
{{end}}

{{- range $item := .ConfigList}}
message {{$item.Name}} {
	{{- $fileName := $item.FileName}}
	{{- range $pos, $field := $item.FieldList}}
	{{protofieldmodifier $field.Type}}{{protofieldtype $fileName $field.Type}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
{{- end}}
}

message {{$item.Name}}Ary { 
	repeated {{$item.Name}} Ary = 1;
}
{{end}}
`

var (
	ProtoTpl *template.Template
	CodeTpl  *template.Template
	IndexTpl *template.Template
)

func init() {
	funcs := template.FuncMap{
		"sub":                base.Sub,
		"add":                base.Add,
		"protofieldmodifier": protoFieldModifier,
		"protofieldtype":     protoFieldType,
	}
	ProtoTpl = template.Must(template.New("ProtoTpl").Funcs(funcs).Parse(protoTpl))
	IndexTpl = template.Must(template.New("IndexTpl").Funcs(funcs).Parse(indexTpl))
	CodeTpl = template.Must(template.New("CodeTpl").Funcs(funcs).Parse(codeTpl))
}
