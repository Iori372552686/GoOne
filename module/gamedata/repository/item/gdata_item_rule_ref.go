// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ItemRuleRefConfig
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

// IItemRuleRef 是 ItemRuleRefConfig 的查询接口。
// 包级单例 ItemRuleRef 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IItemRuleRef interface {
	// 基础查询
	GetHead() *protocol.ItemRuleRefConfig
	GetAll() []*protocol.ItemRuleRefConfig
	Count() int
	Range(fn func(*protocol.ItemRuleRefConfig) bool)
	Find(fn func(*protocol.ItemRuleRefConfig) bool) *protocol.ItemRuleRefConfig
	Filter(fn func(*protocol.ItemRuleRefConfig) bool) []*protocol.ItemRuleRefConfig
	// 主键索引（唯一）
	GetByItemId(ItemId int32) *protocol.ItemRuleRefConfig
	MustGetByItemId(ItemId int32) *protocol.ItemRuleRefConfig
	HasByItemId(ItemId int32) bool
	GetMapItemId() map[int32]*protocol.ItemRuleRefConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ItemRuleRefSnapshot struct {
	list      []*protocol.ItemRuleRefConfig
	mapItemId map[int32]*protocol.ItemRuleRefConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ItemRuleRef 是 ItemRuleRefConfig 的包级单例，实现 IItemRuleRef。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ItemRuleRef IItemRuleRef = &ItemRuleRefImpl{}

var ItemRuleRefPtr atomic.Pointer[ItemRuleRefSnapshot]

type ItemRuleRefImpl struct{}

func (c *ItemRuleRefImpl) load() *ItemRuleRefSnapshot {
	return ItemRuleRefPtr.Load()
}

func init() {
	gamedata.Register("ItemRuleRefConfig", parseItemRuleRef)
}

func parseItemRuleRef(buf string) error {
	data := &protocol.ItemRuleRefConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ItemRuleRefSnapshot{
		list:      data.Ary,
		mapItemId: make(map[int32]*protocol.ItemRuleRefConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapItemId[item.ItemId]; exists {
			return fmt.Errorf("ItemRuleRefConfig 重复主键 ItemId=%v", item.ItemId)
		}
		s.mapItemId[item.ItemId] = item
	}

	ItemRuleRefPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ItemRuleRefImpl) GetHead() *protocol.ItemRuleRefConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ItemRuleRefImpl) GetAll() []*protocol.ItemRuleRefConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ItemRuleRefConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ItemRuleRefImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ItemRuleRefImpl) Range(fn func(*protocol.ItemRuleRefConfig) bool) {
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

func (c *ItemRuleRefImpl) Find(fn func(*protocol.ItemRuleRefConfig) bool) *protocol.ItemRuleRefConfig {
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

func (c *ItemRuleRefImpl) Filter(fn func(*protocol.ItemRuleRefConfig) bool) []*protocol.ItemRuleRefConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ItemRuleRefConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetItemRuleRefHead() *protocol.ItemRuleRefConfig            { return ItemRuleRef.GetHead() }
func GetItemRuleRefAll() []*protocol.ItemRuleRefConfig           { return ItemRuleRef.GetAll() }
func CountItemRuleRef() int                                      { return ItemRuleRef.Count() }
func RangeItemRuleRef(fn func(*protocol.ItemRuleRefConfig) bool) { ItemRuleRef.Range(fn) }
func FindItemRuleRef(fn func(*protocol.ItemRuleRefConfig) bool) *protocol.ItemRuleRefConfig {
	return ItemRuleRef.Find(fn)
}
func FilterItemRuleRef(fn func(*protocol.ItemRuleRefConfig) bool) []*protocol.ItemRuleRefConfig {
	return ItemRuleRef.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ItemRuleRefImpl) GetByItemId(ItemId int32) *protocol.ItemRuleRefConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapItemId[ItemId]
}

// MustGetByItemId 未命中时 panic，用于配置必须存在的确定性场景
func (c *ItemRuleRefImpl) MustGetByItemId(ItemId int32) *protocol.ItemRuleRefConfig {
	v := c.GetByItemId(ItemId)
	if v == nil {
		panic(fmt.Sprintf("ItemRuleRefConfig 主键 ItemId=%v 不存在", ItemId))
	}
	return v
}

func (c *ItemRuleRefImpl) HasByItemId(ItemId int32) bool {
	return c.GetByItemId(ItemId) != nil
}

func (c *ItemRuleRefImpl) GetMapItemId() map[int32]*protocol.ItemRuleRefConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.ItemRuleRefConfig, len(s.mapItemId))
	for k, v := range s.mapItemId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetItemRuleRefByItemId(ItemId int32) *protocol.ItemRuleRefConfig {
	return ItemRuleRef.GetByItemId(ItemId)
}
func MustGetItemRuleRefByItemId(ItemId int32) *protocol.ItemRuleRefConfig {
	return ItemRuleRef.MustGetByItemId(ItemId)
}
func HasItemRuleRefByItemId(ItemId int32) bool {
	return ItemRuleRef.HasByItemId(ItemId)
}
func GetItemRuleRefMapItemId() map[int32]*protocol.ItemRuleRefConfig {
	return ItemRuleRef.GetMapItemId()
}
