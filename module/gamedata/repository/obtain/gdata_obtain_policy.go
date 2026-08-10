// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ObtainPolicyConfig
// ============================================================================

package obtain

import (
	"fmt"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
//  接口定义
// ---------------------------------------------------------------------------

// IObtainPolicy 是 ObtainPolicyConfig 的查询接口。
// 包级单例 ObtainPolicy 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IObtainPolicy interface {
	// 基础查询
	GetHead() *protocol.ObtainPolicyConfig
	GetAll() []*protocol.ObtainPolicyConfig
	Count() int
	Range(fn func(*protocol.ObtainPolicyConfig) bool)
	Find(fn func(*protocol.ObtainPolicyConfig) bool) *protocol.ObtainPolicyConfig
	Filter(fn func(*protocol.ObtainPolicyConfig) bool) []*protocol.ObtainPolicyConfig
	// 主键索引（唯一）
	GetBySource(Source string) *protocol.ObtainPolicyConfig
	MustGetBySource(Source string) *protocol.ObtainPolicyConfig
	HasBySource(Source string) bool
	GetMapSource() map[string]*protocol.ObtainPolicyConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ObtainPolicySnapshot struct {
	list      []*protocol.ObtainPolicyConfig
	mapSource map[string]*protocol.ObtainPolicyConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ObtainPolicy 是 ObtainPolicyConfig 的包级单例，实现 IObtainPolicy。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ObtainPolicy IObtainPolicy = &ObtainPolicyImpl{}

var ObtainPolicyPtr atomic.Pointer[ObtainPolicySnapshot]

type ObtainPolicyImpl struct{}

func (c *ObtainPolicyImpl) load() *ObtainPolicySnapshot {
	return ObtainPolicyPtr.Load()
}

func init() {
	gamedata.Register("ObtainPolicyConfig", parseObtainPolicy)
}

func parseObtainPolicy(buf string) error {
	data := &protocol.ObtainPolicyConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ObtainPolicySnapshot{
		list:      data.Ary,
		mapSource: make(map[string]*protocol.ObtainPolicyConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapSource[item.Source]; exists {
			return fmt.Errorf("ObtainPolicyConfig 重复主键 Source=%v", item.Source)
		}
		s.mapSource[item.Source] = item
	}

	ObtainPolicyPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ObtainPolicyImpl) GetHead() *protocol.ObtainPolicyConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ObtainPolicyImpl) GetAll() []*protocol.ObtainPolicyConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ObtainPolicyConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ObtainPolicyImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ObtainPolicyImpl) Range(fn func(*protocol.ObtainPolicyConfig) bool) {
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

func (c *ObtainPolicyImpl) Find(fn func(*protocol.ObtainPolicyConfig) bool) *protocol.ObtainPolicyConfig {
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

func (c *ObtainPolicyImpl) Filter(fn func(*protocol.ObtainPolicyConfig) bool) []*protocol.ObtainPolicyConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ObtainPolicyConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetObtainPolicyHead() *protocol.ObtainPolicyConfig            { return ObtainPolicy.GetHead() }
func GetObtainPolicyAll() []*protocol.ObtainPolicyConfig           { return ObtainPolicy.GetAll() }
func CountObtainPolicy() int                                       { return ObtainPolicy.Count() }
func RangeObtainPolicy(fn func(*protocol.ObtainPolicyConfig) bool) { ObtainPolicy.Range(fn) }
func FindObtainPolicy(fn func(*protocol.ObtainPolicyConfig) bool) *protocol.ObtainPolicyConfig {
	return ObtainPolicy.Find(fn)
}
func FilterObtainPolicy(fn func(*protocol.ObtainPolicyConfig) bool) []*protocol.ObtainPolicyConfig {
	return ObtainPolicy.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ObtainPolicyImpl) GetBySource(Source string) *protocol.ObtainPolicyConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapSource[Source]
}

// MustGetBySource 未命中时 panic，用于配置必须存在的确定性场景
func (c *ObtainPolicyImpl) MustGetBySource(Source string) *protocol.ObtainPolicyConfig {
	v := c.GetBySource(Source)
	if v == nil {
		panic(fmt.Sprintf("ObtainPolicyConfig 主键 Source=%v 不存在", Source))
	}
	return v
}

func (c *ObtainPolicyImpl) HasBySource(Source string) bool {
	return c.GetBySource(Source) != nil
}

func (c *ObtainPolicyImpl) GetMapSource() map[string]*protocol.ObtainPolicyConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[string]*protocol.ObtainPolicyConfig, len(s.mapSource))
	for k, v := range s.mapSource {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetObtainPolicyBySource(Source string) *protocol.ObtainPolicyConfig {
	return ObtainPolicy.GetBySource(Source)
}
func MustGetObtainPolicyBySource(Source string) *protocol.ObtainPolicyConfig {
	return ObtainPolicy.MustGetBySource(Source)
}
func HasObtainPolicyBySource(Source string) bool {
	return ObtainPolicy.HasBySource(Source)
}
func GetObtainPolicyMapSource() map[string]*protocol.ObtainPolicyConfig {
	return ObtainPolicy.GetMapSource()
}
