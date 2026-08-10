package templ

const codeTpl = `
{{/* 定义全局变量  */}}
{{$type := .Name}}
{{$prefix := .Prefix}}
{{$indexs := .IndexList}}
{{$indexMap := .Indexs}}
{{$pkg := .PbPkg}}

// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: {{$type}}
// ============================================================================

package {{.Pkg}}

import (
{{- if index $indexMap 3}}
	"fmt"
{{- end}}
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	{{.PbPkg}} "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
//  接口定义
// ---------------------------------------------------------------------------

// I{{$prefix}} 是 {{$type}} 的查询接口。
// 包级单例 {{$prefix}} 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type I{{$prefix}} interface {
	// 基础查询
	GetHead() *{{$pkg}}.{{$type}}
	GetAll() []*{{$pkg}}.{{$type}}
	Count() int
	Range(fn func(*{{$pkg}}.{{$type}}) bool)
	Find(fn func(*{{$pkg}}.{{$type}}) bool) *{{$pkg}}.{{$type}}
	Filter(fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}}

{{- range $index := $indexs}}
  {{- if eq $index.Type.ValueOf 3}}    {{/*ValueOfMap*/}}
	// 主键索引（唯一）
	GetBy{{$index.Name}}({{$index.Arg ","}}) *{{$pkg}}.{{$type}}
	MustGetBy{{$index.Name}}({{$index.Arg ","}}) *{{$pkg}}.{{$type}}
	HasBy{{$index.Name}}({{$index.Arg ","}}) bool
	GetMap{{$index.Name}}() map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}}
  {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}
	// 分组索引（一对多）
	GroupBy{{$index.Name}}({{$index.Arg ","}}) []*{{$pkg}}.{{$type}}
	GroupBy{{$index.Name}}Func({{$index.Arg ","}}, fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}}
	GetMap{{$index.Name}}() map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}}
  {{- end}}
{{- end}}
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type {{$prefix}}Snapshot struct {
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

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// {{$prefix}} 是 {{$type}} 的包级单例，实现 I{{$prefix}}。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var {{$prefix}} I{{$prefix}} = &{{$prefix}}Impl{}

var {{$prefix}}Ptr atomic.Pointer[{{$prefix}}Snapshot]

type {{$prefix}}Impl struct{}

func (c *{{$prefix}}Impl) load() *{{$prefix}}Snapshot {
	return {{$prefix}}Ptr.Load()
}

func init() {
	gamedata.Register("{{$type}}", parse{{$prefix}})
}

func parse{{$prefix}}(buf string) error {
	data := &{{$pkg}}.{{$type}}Ary{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &{{$prefix}}Snapshot{
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
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.map{{$index.Name}}[{{$key}}]; exists {
			return fmt.Errorf("{{$type}} 重复主键 {{$index.Name}}=%v", {{$key}})
		}
		s.map{{$index.Name}}[{{$key}}] = item
        {{- else if eq $index.Type.TypeOf 3}}
		_key{{$index.Name}} := {{$index.Type.Name}}{ {{$index.KeyStructInit "item"}} }
		if _, exists := s.map{{$index.Name}}[_key{{$index.Name}}]; exists {
			return fmt.Errorf("{{$type}} 重复复合主键 {{$index.Name}}")
		}
		s.map{{$index.Name}}[_key{{$index.Name}}] = item
        {{- end}}
      {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}
        {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
		s.group{{$index.Name}}[{{$key}}] = append(s.group{{$index.Name}}[{{$key}}], item)
        {{- else if eq $index.Type.TypeOf 3}}
		_key{{$index.Name}} := {{$index.Type.Name}}{ {{$index.KeyStructInit "item"}} }
		s.group{{$index.Name}}[_key{{$index.Name}}] = append(s.group{{$index.Name}}[_key{{$index.Name}}], item)
        {{- end}}
      {{- end}}
    {{- end}}
	}
{{end}}
	{{$prefix}}Ptr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------
{{if index $indexMap 2}}
{{$first := index (index $indexMap 2) 0}}
{{if $first -}}

func (c *{{$prefix}}Impl) GetHead() *{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *{{$prefix}}Impl) GetAll() []*{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*{{$pkg}}.{{$type}}, len(s.list))
	copy(out, s.list)
	return out
}

func (c *{{$prefix}}Impl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *{{$prefix}}Impl) Range(fn func(*{{$pkg}}.{{$type}}) bool) {
	s := c.load()
	if s == nil {
		return
	}
	for _, item := range s.list {
		if !fn(item) {
			return
		}
	}
}

func (c *{{$prefix}}Impl) Find(fn func(*{{$pkg}}.{{$type}}) bool) *{{$pkg}}.{{$type}} {
	s := c.load()
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

func (c *{{$prefix}}Impl) Filter(fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}} {
	s := c.load()
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

// 包级函数代理（兼容现有调用）
func Get{{$prefix}}Head() *{{$pkg}}.{{$type}}            { return {{$prefix}}.GetHead() }
func Get{{$prefix}}All() []*{{$pkg}}.{{$type}}           { return {{$prefix}}.GetAll() }
func Count{{$prefix}}() int                              { return {{$prefix}}.Count() }
func Range{{$prefix}}(fn func(*{{$pkg}}.{{$type}}) bool) { {{$prefix}}.Range(fn) }
func Find{{$prefix}}(fn func(*{{$pkg}}.{{$type}}) bool) *{{$pkg}}.{{$type}} {
	return {{$prefix}}.Find(fn)
}
func Filter{{$prefix}}(fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}} {
	return {{$prefix}}.Filter(fn)
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

func (c *{{$prefix}}Impl) GetBy{{$index.Name}}({{$arg}}) *{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
    {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
	return s.map{{$index.Name}}[{{$key}}]
    {{- else if eq $index.Type.TypeOf 3}}
	return s.map{{$index.Name}}[{{$index.Type.Name}}{ {{$index.KeyStructInit ""}} }]
    {{- end}}
}

// MustGetBy{{$index.Name}} 未命中时 panic，用于配置必须存在的确定性场景
func (c *{{$prefix}}Impl) MustGetBy{{$index.Name}}({{$arg}}) *{{$pkg}}.{{$type}} {
	v := c.GetBy{{$index.Name}}({{$key}})
	if v == nil {
		panic(fmt.Sprintf("{{$type}} 主键 {{$index.Name}}={{range $i, $f := $index.List}}{{if $i}}, {{end}}%v{{end}} 不存在"{{range $index.List}}, {{.Name}}{{end}}))
	}
	return v
}

func (c *{{$prefix}}Impl) HasBy{{$index.Name}}({{$arg}}) bool {
	return c.GetBy{{$index.Name}}({{$key}}) != nil
}

func (c *{{$prefix}}Impl) GetMap{{$index.Name}}() map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}}, len(s.map{{$index.Name}}))
	for k, v := range s.map{{$index.Name}} {
		out[k] = v
	}
	return out
}

// 包级函数代理
func Get{{$prefix}}By{{$index.Name}}({{$arg}}) *{{$pkg}}.{{$type}} {
	return {{$prefix}}.GetBy{{$index.Name}}({{$key}})
}
func MustGet{{$prefix}}By{{$index.Name}}({{$arg}}) *{{$pkg}}.{{$type}} {
	return {{$prefix}}.MustGetBy{{$index.Name}}({{$key}})
}
func Has{{$prefix}}By{{$index.Name}}({{$arg}}) bool {
	return {{$prefix}}.HasBy{{$index.Name}}({{$key}})
}
func Get{{$prefix}}Map{{$index.Name}}() map[{{$index.Type.Name}}]*{{$pkg}}.{{$type}} {
	return {{$prefix}}.GetMap{{$index.Name}}()
}
  {{- else if eq $index.Type.ValueOf 4}}    {{/*ValueOfGroup*/}}

func (c *{{$prefix}}Impl) GroupBy{{$index.Name}}({{$arg}}) []*{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
    {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
	src := s.group{{$index.Name}}[{{$key}}]
    {{- else if eq $index.Type.TypeOf 3}}
	src := s.group{{$index.Name}}[{{$index.Type.Name}}{ {{$index.KeyStructInit ""}} }]
    {{- end}}
	if len(src) == 0 {
		return nil
	}
	out := make([]*{{$pkg}}.{{$type}}, len(src))
	copy(out, src)
	return out
}

// GroupBy{{$index.Name}}Func 在分组内二次筛选：先按索引缩小到该 key 的记录，再回调过滤。
// 比全表 Filter 高效（O 组内数 vs O 全表数）。
func (c *{{$prefix}}Impl) GroupBy{{$index.Name}}Func({{$arg}}, fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
    {{- if or (eq $index.Type.TypeOf 1) (eq $index.Type.TypeOf 2)}}
	src := s.group{{$index.Name}}[{{$key}}]
    {{- else if eq $index.Type.TypeOf 3}}
	src := s.group{{$index.Name}}[{{$index.Type.Name}}{ {{$index.KeyStructInit ""}} }]
    {{- end}}
	var out []*{{$pkg}}.{{$type}}
	for _, item := range src {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

func (c *{{$prefix}}Impl) GetMap{{$index.Name}}() map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}} {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}}, len(s.group{{$index.Name}}))
	for k, v := range s.group{{$index.Name}} {
		cp := make([]*{{$pkg}}.{{$type}}, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// 包级函数代理
func Group{{$prefix}}By{{$index.Name}}({{$arg}}) []*{{$pkg}}.{{$type}} {
	return {{$prefix}}.GroupBy{{$index.Name}}({{$key}})
}
func Group{{$prefix}}By{{$index.Name}}Func({{$arg}}, fn func(*{{$pkg}}.{{$type}}) bool) []*{{$pkg}}.{{$type}} {
	return {{$prefix}}.GroupBy{{$index.Name}}Func({{$key}}, fn)
}
func Get{{$prefix}}Map{{$index.Name}}() map[{{$index.Type.Name}}][]*{{$pkg}}.{{$type}} {
	return {{$prefix}}.GetMap{{$index.Name}}()
}
  {{- end}}
{{- end}}
`
