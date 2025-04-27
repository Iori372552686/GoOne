package templ

const indexTpl = `
/*
* 本代码由xlsx工具生成，请勿手动修改
*/

package {{.Pkg}}

{{define "member" -}}
	{{- if gt . 0 -}}
		T{{.}} T{{.}}
		{{template "member" sub . 1}}
	{{- end -}}
{{- end -}}

{{- define "type" -}}
	{{- if gt . 0 -}} T{{.}} {{- if gt . 1 -}},{{- end -}} {{template "type" sub . 1}} {{- end -}}
{{- end -}}

{{range $pos := .IndexList -}}	
	{{if gt $pos 1 -}}
type Index{{$pos}}[{{template "type" $pos}} any] struct {
	{{template "member" $pos}}
}	
	{{- end}}
{{- end}}
`
