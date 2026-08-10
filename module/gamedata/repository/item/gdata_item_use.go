// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ItemUseConfig
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

// IItemUse 是 ItemUseConfig 的查询接口。
// 包级单例 ItemUse 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IItemUse interface {
	// 基础查询
	GetHead() *protocol.ItemUseConfig
	GetAll() []*protocol.ItemUseConfig
	Count() int
	Range(fn func(*protocol.ItemUseConfig) bool)
	Find(fn func(*protocol.ItemUseConfig) bool) *protocol.ItemUseConfig
	Filter(fn func(*protocol.ItemUseConfig) bool) []*protocol.ItemUseConfig
	// 主键索引（唯一）
	GetById(Id int32) *protocol.ItemUseConfig
	MustGetById(Id int32) *protocol.ItemUseConfig
	HasById(Id int32) bool
	GetMapId() map[int32]*protocol.ItemUseConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ItemUseSnapshot struct {
	list  []*protocol.ItemUseConfig
	mapId map[int32]*protocol.ItemUseConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ItemUse 是 ItemUseConfig 的包级单例，实现 IItemUse。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ItemUse IItemUse = &ItemUseImpl{}

var ItemUsePtr atomic.Pointer[ItemUseSnapshot]

type ItemUseImpl struct{}

func (c *ItemUseImpl) load() *ItemUseSnapshot {
	return ItemUsePtr.Load()
}

func init() {
	gamedata.Register("ItemUseConfig", parseItemUse)
}

func parseItemUse(buf string) error {
	data := &protocol.ItemUseConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ItemUseSnapshot{
		list:  data.Ary,
		mapId: make(map[int32]*protocol.ItemUseConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapId[item.Id]; exists {
			return fmt.Errorf("ItemUseConfig 重复主键 Id=%v", item.Id)
		}
		s.mapId[item.Id] = item
	}

	ItemUsePtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ItemUseImpl) GetHead() *protocol.ItemUseConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ItemUseImpl) GetAll() []*protocol.ItemUseConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ItemUseConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ItemUseImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ItemUseImpl) Range(fn func(*protocol.ItemUseConfig) bool) {
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

func (c *ItemUseImpl) Find(fn func(*protocol.ItemUseConfig) bool) *protocol.ItemUseConfig {
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

func (c *ItemUseImpl) Filter(fn func(*protocol.ItemUseConfig) bool) []*protocol.ItemUseConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ItemUseConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetItemUseHead() *protocol.ItemUseConfig            { return ItemUse.GetHead() }
func GetItemUseAll() []*protocol.ItemUseConfig           { return ItemUse.GetAll() }
func CountItemUse() int                                  { return ItemUse.Count() }
func RangeItemUse(fn func(*protocol.ItemUseConfig) bool) { ItemUse.Range(fn) }
func FindItemUse(fn func(*protocol.ItemUseConfig) bool) *protocol.ItemUseConfig {
	return ItemUse.Find(fn)
}
func FilterItemUse(fn func(*protocol.ItemUseConfig) bool) []*protocol.ItemUseConfig {
	return ItemUse.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ItemUseImpl) GetById(Id int32) *protocol.ItemUseConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapId[Id]
}

// MustGetById 未命中时 panic，用于配置必须存在的确定性场景
func (c *ItemUseImpl) MustGetById(Id int32) *protocol.ItemUseConfig {
	v := c.GetById(Id)
	if v == nil {
		panic(fmt.Sprintf("ItemUseConfig 主键 Id=%v 不存在", Id))
	}
	return v
}

func (c *ItemUseImpl) HasById(Id int32) bool {
	return c.GetById(Id) != nil
}

func (c *ItemUseImpl) GetMapId() map[int32]*protocol.ItemUseConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.ItemUseConfig, len(s.mapId))
	for k, v := range s.mapId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetItemUseById(Id int32) *protocol.ItemUseConfig {
	return ItemUse.GetById(Id)
}
func MustGetItemUseById(Id int32) *protocol.ItemUseConfig {
	return ItemUse.MustGetById(Id)
}
func HasItemUseById(Id int32) bool {
	return ItemUse.HasById(Id)
}
func GetItemUseMapId() map[int32]*protocol.ItemUseConfig {
	return ItemUse.GetMapId()
}
