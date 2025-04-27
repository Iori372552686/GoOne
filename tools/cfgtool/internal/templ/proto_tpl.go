package templ

import (
	"text/template"

	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

const protoTpl = `
/*
* 本代码由xlsx工具生成，请勿手动修改
*/

syntax = "proto3";

package g1.protocol;

option go_package = "./g1_protocol";

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

{{- range $item := .StructList}}
message {{$item.Name}} {
	{{- range $pos, $field := $item.FieldList}}
		{{- if eq $field.Type.ValueOf 1}}
	{{$field.Type.Name}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
		{{- else if eq $field.Type.ValueOf 2}} 
	repeated {{$field.Type.Name}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
		{{- end}} 
{{- end}}
}
{{end}}

{{- range $item := .ConfigList}}
message {{$item.Name}} {
	{{- range $pos, $field := $item.FieldList}}
		{{- if eq $field.Type.ValueOf 1}}
	{{$field.Type.Name}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
		{{- else if eq $field.Type.ValueOf 2}} 
	repeated {{$field.Type.Name}} {{$field.Name}} = {{add $pos 1}}; // {{$field.Desc}}
		{{- end}} 
{{- end}}
}

message {{$item.Name}}Ary { repeated {{$item.Name}} Ary = 1; }
{{end}}
`

var (
	ProtoTpl *template.Template
	CodeTpl  *template.Template
	IndexTpl *template.Template
)

func init() {
	funcs := template.FuncMap{
		"sub": base.Sub,
		"add": base.Add,
	}
	ProtoTpl = template.Must(template.New("ProtoTpl").Funcs(funcs).Parse(protoTpl))
	IndexTpl = template.Must(template.New("IndexTpl").Funcs(funcs).Parse(indexTpl))
	CodeTpl = template.Must(template.New("CodeTpl").Funcs(funcs).Parse(codeTpl))
}
