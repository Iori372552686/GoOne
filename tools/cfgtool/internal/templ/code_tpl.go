package templ

const codeTpl = `
{{/* 定义全局变量  */}}
{{$type := .Name}} 
{{$indexs := .IndexList}}
{{$indexMap := .Indexs}}
{{$pkg := .PbPkg}}

// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: {{$type}}
// ============================================================================

package {{.Pkg}}

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	{{.PbPkg}} "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type snapshot struct {
{{- range $index := $indexs -}}
    {{- if eq $index.Type.ValueOf 2}}         {{/*ValueOfList*/}}
	list []*{{$pkg}}.{{$type}}
    {{- else if eq $index.Type.ValueOf 3}}    {{/*ValueOfMap*/}}
	map{{$index.Name}} map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}}
    {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}
	group{{$index.Name}} map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}}
    {{- end -}}
{{- end}}
}

var ptr atomic.Pointer[snapshot]

func load() *snapshot {
	return ptr.Load()
}

// ---------------------------------------------------------------------------
//  注册 & 加载
// ---------------------------------------------------------------------------

func init() {
	gamedata.Register("{{$type}}", parse)
}

func parse(buf string) error {
	data := &{{$pkg}}.{{$type}}Ary{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &snapshot{
{{- range $index := $indexs}}
    {{- if eq $index.Type.ValueOf 2}}
		list: data.Ary,
    {{- else if eq $index.Type.ValueOf 3}}
		map{{$index.Name}}: make(map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}}, len(data.Ary)),
    {{- else if eq $index.Type.ValueOf 4}}
		group{{$index.Name}}: make(map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}}),
    {{- end}}
{{- end}}
	}

{{if or (index $indexMap 3) (index $indexMap 4)}}
	for _, item := range data.Ary {
    {{- range $index := $indexs}}
      {{- $key := $index.Value "item" ","}}
      {{- if eq $index.Type.ValueOf 3}}    {{/*ValueOfMap*/}}
        {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
		s.map{{$index.Name}}[{{$key}}] = item
        {{- else if eq $index.Type.TypeOf 3}}
		s.map{{$index.Name}}[{{$index.Type.Name}}{ {{$key}} }] = item
        {{- end}}
      {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}
        {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
		s.group{{$index.Name}}[{{$key}}] = append(s.group{{$index.Name}}[{{$key}}], item)
        {{- else if eq $index.Type.TypeOf 3}}
		s.group{{$index.Name}}[{{$index.Type.Name}}{ {{$key}} }] = append(s.group{{$index.Name}}[{{$index.Type.Name}}{ {{$key}} }], item)
        {{- end}}
      {{- end}}
    {{- end}}
	}
{{end}}
	ptr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------
{{if index $indexMap 2}}
{{$index := index (index $indexMap 2) 0}}
{{if $index -}}

// GetHead 返回第一条记录，无数据时返回 nil
func GetHead() *{{$pkg}}.{{$type}} {
	s := load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

// GetAll 返回全部记录的拷贝切片
func GetAll() []*{{$pkg}}.{{$type}} {
	s := load()
	if s == nil {
		return nil
	}
	out := make([]*{{$pkg}}.{{$type}}, len(s.list))
	copy(out, s.list)
	return out
}

// Count 返回记录总数
func Count() int {
	s := load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

// ---------------------------------------------------------------------------
//  遍历
// ---------------------------------------------------------------------------

// Range 遍历所有记录，fn 返回 false 时提前终止
func Range(fn func(*{{$pkg}}.{{$type}}) bool) {
	s := load()
	if s == nil {
		return
	}
	for _, item := range s.list {
		if !fn(item) {
			return
		}
	}
}

// ---------------------------------------------------------------------------
//  条件查询
// ---------------------------------------------------------------------------

// Find 返回第一个满足条件的记录，无匹配返回 nil
func Find(fn func(*{{$pkg}}.{{$type}}) bool) *{{$pkg}}.{{$type}} {
	s := load()
	if s == nil {
		return nil
	}
	for _, item := range s.list {
		if fn(item) {
			return item
		}
	}
	return nil
}

// Filter 返回所有满足条件的记录
func Filter(fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}} {
	s := load()
	if s == nil {
		return nil
	}
	var out []*{{$pkg}}.{{$type}}
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}
{{- end}}
{{- end}}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------
{{- range $index := $indexs}}
  {{- $arg := $index.Arg ","}}
  {{- $key := $index.Value "" ","}}
  {{- if eq $index.Type.ValueOf 3}}    {{/*ValueOfMap*/}}

// GetBy{{$index.Name}} 按索引精确查找
func GetBy{{$index.Name}}({{$arg}}) *{{$pkg}}.{{$type}} {
	s := load()
	if s == nil {
		return nil
	}
    {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
	return s.map{{$index.Name}}[{{$key}}]
    {{- else if eq $index.Type.TypeOf 3}}
	return s.map{{$index.Name}}[{{$index.Type.Name}}{ {{$key}} }]
    {{- end}}
}
  {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}

// GroupBy{{$index.Name}} 按索引分组查找
func GroupBy{{$index.Name}}({{$arg}}) []*{{$pkg}}.{{$type}} {
	s := load()
	if s == nil {
		return nil
	}
    {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
	src := s.group{{$index.Name}}[{{$key}}]
    {{- else if eq $index.Type.TypeOf 3}}
	src := s.group{{$index.Name}}[{{$index.Type.Name}}{ {{$key}} }]
    {{- end}}
	if len(src) == 0 {
		return nil
	}
	out := make([]*{{$pkg}}.{{$type}}, len(src))
	copy(out, src)
	return out
}
  {{- end}}
{{- end}}
`
