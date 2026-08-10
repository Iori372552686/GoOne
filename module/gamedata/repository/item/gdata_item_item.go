// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ItemConfig
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

// IItem 是 ItemConfig 的查询接口。
// 包级单例 Item 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IItem interface {
	// 基础查询
	GetHead() *protocol.ItemConfig
	GetAll() []*protocol.ItemConfig
	Count() int
	Range(fn func(*protocol.ItemConfig) bool)
	Find(fn func(*protocol.ItemConfig) bool) *protocol.ItemConfig
	Filter(fn func(*protocol.ItemConfig) bool) []*protocol.ItemConfig
	// 主键索引（唯一）
	GetByItemId(ItemId int32) *protocol.ItemConfig
	MustGetByItemId(ItemId int32) *protocol.ItemConfig
	HasByItemId(ItemId int32) bool
	GetMapItemId() map[int32]*protocol.ItemConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ItemSnapshot struct {
	list      []*protocol.ItemConfig
	mapItemId map[int32]*protocol.ItemConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// Item 是 ItemConfig 的包级单例，实现 IItem。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var Item IItem = &ItemImpl{}

var ItemPtr atomic.Pointer[ItemSnapshot]

type ItemImpl struct{}

func (c *ItemImpl) load() *ItemSnapshot {
	return ItemPtr.Load()
}

func init() {
	gamedata.Register("ItemConfig", parseItem)
}

func parseItem(buf string) error {
	data := &protocol.ItemConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ItemSnapshot{
		list:      data.Ary,
		mapItemId: make(map[int32]*protocol.ItemConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapItemId[item.ItemId]; exists {
			return fmt.Errorf("ItemConfig 重复主键 ItemId=%v", item.ItemId)
		}
		s.mapItemId[item.ItemId] = item
	}

	ItemPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ItemImpl) GetHead() *protocol.ItemConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ItemImpl) GetAll() []*protocol.ItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ItemConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ItemImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ItemImpl) Range(fn func(*protocol.ItemConfig) bool) {
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

func (c *ItemImpl) Find(fn func(*protocol.ItemConfig) bool) *protocol.ItemConfig {
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

func (c *ItemImpl) Filter(fn func(*protocol.ItemConfig) bool) []*protocol.ItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ItemConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetItemHead() *protocol.ItemConfig            { return Item.GetHead() }
func GetItemAll() []*protocol.ItemConfig           { return Item.GetAll() }
func CountItem() int                               { return Item.Count() }
func RangeItem(fn func(*protocol.ItemConfig) bool) { Item.Range(fn) }
func FindItem(fn func(*protocol.ItemConfig) bool) *protocol.ItemConfig {
	return Item.Find(fn)
}
func FilterItem(fn func(*protocol.ItemConfig) bool) []*protocol.ItemConfig {
	return Item.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ItemImpl) GetByItemId(ItemId int32) *protocol.ItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapItemId[ItemId]
}

// MustGetByItemId 未命中时 panic，用于配置必须存在的确定性场景
func (c *ItemImpl) MustGetByItemId(ItemId int32) *protocol.ItemConfig {
	v := c.GetByItemId(ItemId)
	if v == nil {
		panic(fmt.Sprintf("ItemConfig 主键 ItemId=%v 不存在", ItemId))
	}
	return v
}

func (c *ItemImpl) HasByItemId(ItemId int32) bool {
	return c.GetByItemId(ItemId) != nil
}

func (c *ItemImpl) GetMapItemId() map[int32]*protocol.ItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.ItemConfig, len(s.mapItemId))
	for k, v := range s.mapItemId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetItemByItemId(ItemId int32) *protocol.ItemConfig {
	return Item.GetByItemId(ItemId)
}
func MustGetItemByItemId(ItemId int32) *protocol.ItemConfig {
	return Item.MustGetByItemId(ItemId)
}
func HasItemByItemId(ItemId int32) bool {
	return Item.HasByItemId(ItemId)
}
func GetItemMapItemId() map[int32]*protocol.ItemConfig {
	return Item.GetMapItemId()
}
