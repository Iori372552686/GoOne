// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ConstsGlobalConfig
// ============================================================================

package consts

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

// IConstsGlobal 是 ConstsGlobalConfig 的查询接口。
// 包级单例 ConstsGlobal 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IConstsGlobal interface {
	// 基础查询
	GetHead() *protocol.ConstsGlobalConfig
	GetAll() []*protocol.ConstsGlobalConfig
	Count() int
	Range(fn func(*protocol.ConstsGlobalConfig) bool)
	Find(fn func(*protocol.ConstsGlobalConfig) bool) *protocol.ConstsGlobalConfig
	Filter(fn func(*protocol.ConstsGlobalConfig) bool) []*protocol.ConstsGlobalConfig
	// 主键索引（唯一）
	GetByName(Name string) *protocol.ConstsGlobalConfig
	MustGetByName(Name string) *protocol.ConstsGlobalConfig
	HasByName(Name string) bool
	GetMapName() map[string]*protocol.ConstsGlobalConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ConstsGlobalSnapshot struct {
	list    []*protocol.ConstsGlobalConfig
	mapName map[string]*protocol.ConstsGlobalConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ConstsGlobal 是 ConstsGlobalConfig 的包级单例，实现 IConstsGlobal。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ConstsGlobal IConstsGlobal = &ConstsGlobalImpl{}

var ConstsGlobalPtr atomic.Pointer[ConstsGlobalSnapshot]

type ConstsGlobalImpl struct{}

func (c *ConstsGlobalImpl) load() *ConstsGlobalSnapshot {
	return ConstsGlobalPtr.Load()
}

func init() {
	gamedata.Register("ConstsGlobalConfig", parseConstsGlobal)
}

func parseConstsGlobal(buf string) error {
	data := &protocol.ConstsGlobalConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ConstsGlobalSnapshot{
		list:    data.Ary,
		mapName: make(map[string]*protocol.ConstsGlobalConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapName[item.Name]; exists {
			return fmt.Errorf("ConstsGlobalConfig 重复主键 Name=%v", item.Name)
		}
		s.mapName[item.Name] = item
	}

	ConstsGlobalPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ConstsGlobalImpl) GetHead() *protocol.ConstsGlobalConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ConstsGlobalImpl) GetAll() []*protocol.ConstsGlobalConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ConstsGlobalConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ConstsGlobalImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ConstsGlobalImpl) Range(fn func(*protocol.ConstsGlobalConfig) bool) {
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

func (c *ConstsGlobalImpl) Find(fn func(*protocol.ConstsGlobalConfig) bool) *protocol.ConstsGlobalConfig {
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

func (c *ConstsGlobalImpl) Filter(fn func(*protocol.ConstsGlobalConfig) bool) []*protocol.ConstsGlobalConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ConstsGlobalConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetConstsGlobalHead() *protocol.ConstsGlobalConfig            { return ConstsGlobal.GetHead() }
func GetConstsGlobalAll() []*protocol.ConstsGlobalConfig           { return ConstsGlobal.GetAll() }
func CountConstsGlobal() int                                       { return ConstsGlobal.Count() }
func RangeConstsGlobal(fn func(*protocol.ConstsGlobalConfig) bool) { ConstsGlobal.Range(fn) }
func FindConstsGlobal(fn func(*protocol.ConstsGlobalConfig) bool) *protocol.ConstsGlobalConfig {
	return ConstsGlobal.Find(fn)
}
func FilterConstsGlobal(fn func(*protocol.ConstsGlobalConfig) bool) []*protocol.ConstsGlobalConfig {
	return ConstsGlobal.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ConstsGlobalImpl) GetByName(Name string) *protocol.ConstsGlobalConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapName[Name]
}

// MustGetByName 未命中时 panic，用于配置必须存在的确定性场景
func (c *ConstsGlobalImpl) MustGetByName(Name string) *protocol.ConstsGlobalConfig {
	v := c.GetByName(Name)
	if v == nil {
		panic(fmt.Sprintf("ConstsGlobalConfig 主键 Name=%v 不存在", Name))
	}
	return v
}

func (c *ConstsGlobalImpl) HasByName(Name string) bool {
	return c.GetByName(Name) != nil
}

func (c *ConstsGlobalImpl) GetMapName() map[string]*protocol.ConstsGlobalConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[string]*protocol.ConstsGlobalConfig, len(s.mapName))
	for k, v := range s.mapName {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetConstsGlobalByName(Name string) *protocol.ConstsGlobalConfig {
	return ConstsGlobal.GetByName(Name)
}
func MustGetConstsGlobalByName(Name string) *protocol.ConstsGlobalConfig {
	return ConstsGlobal.MustGetByName(Name)
}
func HasConstsGlobalByName(Name string) bool {
	return ConstsGlobal.HasByName(Name)
}
func GetConstsGlobalMapName() map[string]*protocol.ConstsGlobalConfig {
	return ConstsGlobal.GetMapName()
}
