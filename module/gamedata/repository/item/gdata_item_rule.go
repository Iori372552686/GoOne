// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ItemRuleConfig
// ============================================================================

package item

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

// IItemRule 是 ItemRuleConfig 的查询接口。
// 包级单例 ItemRule 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IItemRule interface {
	// 基础查询
	GetHead() *protocol.ItemRuleConfig
	GetAll() []*protocol.ItemRuleConfig
	Count() int
	Range(fn func(*protocol.ItemRuleConfig) bool)
	Find(fn func(*protocol.ItemRuleConfig) bool) *protocol.ItemRuleConfig
	Filter(fn func(*protocol.ItemRuleConfig) bool) []*protocol.ItemRuleConfig
	// 主键索引（唯一）
	GetById(Id int32) *protocol.ItemRuleConfig
	MustGetById(Id int32) *protocol.ItemRuleConfig
	HasById(Id int32) bool
	GetMapId() map[int32]*protocol.ItemRuleConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ItemRuleSnapshot struct {
	list  []*protocol.ItemRuleConfig
	mapId map[int32]*protocol.ItemRuleConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ItemRule 是 ItemRuleConfig 的包级单例，实现 IItemRule。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ItemRule IItemRule = &ItemRuleImpl{}

var ItemRulePtr atomic.Pointer[ItemRuleSnapshot]

type ItemRuleImpl struct{}

func (c *ItemRuleImpl) load() *ItemRuleSnapshot {
	return ItemRulePtr.Load()
}

func init() {
	gamedata.Register("ItemRuleConfig", parseItemRule)
}

func parseItemRule(buf string) error {
	data := &protocol.ItemRuleConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ItemRuleSnapshot{
		list:  data.Ary,
		mapId: make(map[int32]*protocol.ItemRuleConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapId[item.Id]; exists {
			return fmt.Errorf("ItemRuleConfig 重复主键 Id=%v", item.Id)
		}
		s.mapId[item.Id] = item
	}

	ItemRulePtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ItemRuleImpl) GetHead() *protocol.ItemRuleConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ItemRuleImpl) GetAll() []*protocol.ItemRuleConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ItemRuleConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ItemRuleImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ItemRuleImpl) Range(fn func(*protocol.ItemRuleConfig) bool) {
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

func (c *ItemRuleImpl) Find(fn func(*protocol.ItemRuleConfig) bool) *protocol.ItemRuleConfig {
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

func (c *ItemRuleImpl) Filter(fn func(*protocol.ItemRuleConfig) bool) []*protocol.ItemRuleConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ItemRuleConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetItemRuleHead() *protocol.ItemRuleConfig            { return ItemRule.GetHead() }
func GetItemRuleAll() []*protocol.ItemRuleConfig           { return ItemRule.GetAll() }
func CountItemRule() int                                   { return ItemRule.Count() }
func RangeItemRule(fn func(*protocol.ItemRuleConfig) bool) { ItemRule.Range(fn) }
func FindItemRule(fn func(*protocol.ItemRuleConfig) bool) *protocol.ItemRuleConfig {
	return ItemRule.Find(fn)
}
func FilterItemRule(fn func(*protocol.ItemRuleConfig) bool) []*protocol.ItemRuleConfig {
	return ItemRule.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ItemRuleImpl) GetById(Id int32) *protocol.ItemRuleConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapId[Id]
}

// MustGetById 未命中时 panic，用于配置必须存在的确定性场景
func (c *ItemRuleImpl) MustGetById(Id int32) *protocol.ItemRuleConfig {
	v := c.GetById(Id)
	if v == nil {
		panic(fmt.Sprintf("ItemRuleConfig 主键 Id=%v 不存在", Id))
	}
	return v
}

func (c *ItemRuleImpl) HasById(Id int32) bool {
	return c.GetById(Id) != nil
}

func (c *ItemRuleImpl) GetMapId() map[int32]*protocol.ItemRuleConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.ItemRuleConfig, len(s.mapId))
	for k, v := range s.mapId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetItemRuleById(Id int32) *protocol.ItemRuleConfig {
	return ItemRule.GetById(Id)
}
func MustGetItemRuleById(Id int32) *protocol.ItemRuleConfig {
	return ItemRule.MustGetById(Id)
}
func HasItemRuleById(Id int32) bool {
	return ItemRule.HasById(Id)
}
func GetItemRuleMapId() map[int32]*protocol.ItemRuleConfig {
	return ItemRule.GetMapId()
}
