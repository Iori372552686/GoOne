// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: ItemDecomposeConfig
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

// IItemDecompose 是 ItemDecomposeConfig 的查询接口。
// 包级单例 ItemDecompose 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IItemDecompose interface {
	// 基础查询
	GetHead() *protocol.ItemDecomposeConfig
	GetAll() []*protocol.ItemDecomposeConfig
	Count() int
	Range(fn func(*protocol.ItemDecomposeConfig) bool)
	Find(fn func(*protocol.ItemDecomposeConfig) bool) *protocol.ItemDecomposeConfig
	Filter(fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig
	// 主键索引（唯一）
	GetById(Id int32) *protocol.ItemDecomposeConfig
	MustGetById(Id int32) *protocol.ItemDecomposeConfig
	HasById(Id int32) bool
	GetMapId() map[int32]*protocol.ItemDecomposeConfig
	// 分组索引（一对多）
	GroupByItemId(ItemId int32) []*protocol.ItemDecomposeConfig
	GroupByItemIdFunc(ItemId int32, fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig
	GetMapItemId() map[int32][]*protocol.ItemDecomposeConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type ItemDecomposeSnapshot struct {
	list        []*protocol.ItemDecomposeConfig
	mapId       map[int32]*protocol.ItemDecomposeConfig
	groupItemId map[int32][]*protocol.ItemDecomposeConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// ItemDecompose 是 ItemDecomposeConfig 的包级单例，实现 IItemDecompose。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var ItemDecompose IItemDecompose = &ItemDecomposeImpl{}

var ItemDecomposePtr atomic.Pointer[ItemDecomposeSnapshot]

type ItemDecomposeImpl struct{}

func (c *ItemDecomposeImpl) load() *ItemDecomposeSnapshot {
	return ItemDecomposePtr.Load()
}

func init() {
	gamedata.Register("ItemDecomposeConfig", parseItemDecompose)
}

func parseItemDecompose(buf string) error {
	data := &protocol.ItemDecomposeConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &ItemDecomposeSnapshot{
		list:        data.Ary,
		mapId:       make(map[int32]*protocol.ItemDecomposeConfig, len(data.Ary)),
		groupItemId: make(map[int32][]*protocol.ItemDecomposeConfig),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapId[item.Id]; exists {
			return fmt.Errorf("ItemDecomposeConfig 重复主键 Id=%v", item.Id)
		}
		s.mapId[item.Id] = item
		s.groupItemId[item.ItemId] = append(s.groupItemId[item.ItemId], item)
	}

	ItemDecomposePtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *ItemDecomposeImpl) GetHead() *protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *ItemDecomposeImpl) GetAll() []*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.ItemDecomposeConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *ItemDecomposeImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *ItemDecomposeImpl) Range(fn func(*protocol.ItemDecomposeConfig) bool) {
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

func (c *ItemDecomposeImpl) Find(fn func(*protocol.ItemDecomposeConfig) bool) *protocol.ItemDecomposeConfig {
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

func (c *ItemDecomposeImpl) Filter(fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.ItemDecomposeConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetItemDecomposeHead() *protocol.ItemDecomposeConfig            { return ItemDecompose.GetHead() }
func GetItemDecomposeAll() []*protocol.ItemDecomposeConfig           { return ItemDecompose.GetAll() }
func CountItemDecompose() int                                        { return ItemDecompose.Count() }
func RangeItemDecompose(fn func(*protocol.ItemDecomposeConfig) bool) { ItemDecompose.Range(fn) }
func FindItemDecompose(fn func(*protocol.ItemDecomposeConfig) bool) *protocol.ItemDecomposeConfig {
	return ItemDecompose.Find(fn)
}
func FilterItemDecompose(fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig {
	return ItemDecompose.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *ItemDecomposeImpl) GetById(Id int32) *protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapId[Id]
}

// MustGetById 未命中时 panic，用于配置必须存在的确定性场景
func (c *ItemDecomposeImpl) MustGetById(Id int32) *protocol.ItemDecomposeConfig {
	v := c.GetById(Id)
	if v == nil {
		panic(fmt.Sprintf("ItemDecomposeConfig 主键 Id=%v 不存在", Id))
	}
	return v
}

func (c *ItemDecomposeImpl) HasById(Id int32) bool {
	return c.GetById(Id) != nil
}

func (c *ItemDecomposeImpl) GetMapId() map[int32]*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.ItemDecomposeConfig, len(s.mapId))
	for k, v := range s.mapId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetItemDecomposeById(Id int32) *protocol.ItemDecomposeConfig {
	return ItemDecompose.GetById(Id)
}
func MustGetItemDecomposeById(Id int32) *protocol.ItemDecomposeConfig {
	return ItemDecompose.MustGetById(Id)
}
func HasItemDecomposeById(Id int32) bool {
	return ItemDecompose.HasById(Id)
}
func GetItemDecomposeMapId() map[int32]*protocol.ItemDecomposeConfig {
	return ItemDecompose.GetMapId()
}

func (c *ItemDecomposeImpl) GroupByItemId(ItemId int32) []*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	src := s.groupItemId[ItemId]
	if len(src) == 0 {
		return nil
	}
	out := make([]*protocol.ItemDecomposeConfig, len(src))
	copy(out, src)
	return out
}

// GroupByItemIdFunc 在分组内二次筛选：先按索引缩小到该 key 的记录，再回调过滤。
// 比全表 Filter 高效（O 组内数 vs O 全表数）。
func (c *ItemDecomposeImpl) GroupByItemIdFunc(ItemId int32, fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	src := s.groupItemId[ItemId]
	var out []*protocol.ItemDecomposeConfig
	for _, item := range src {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

func (c *ItemDecomposeImpl) GetMapItemId() map[int32][]*protocol.ItemDecomposeConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32][]*protocol.ItemDecomposeConfig, len(s.groupItemId))
	for k, v := range s.groupItemId {
		cp := make([]*protocol.ItemDecomposeConfig, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// 包级函数代理
func GroupItemDecomposeByItemId(ItemId int32) []*protocol.ItemDecomposeConfig {
	return ItemDecompose.GroupByItemId(ItemId)
}
func GroupItemDecomposeByItemIdFunc(ItemId int32, fn func(*protocol.ItemDecomposeConfig) bool) []*protocol.ItemDecomposeConfig {
	return ItemDecompose.GroupByItemIdFunc(ItemId, fn)
}
func GetItemDecomposeMapItemId() map[int32][]*protocol.ItemDecomposeConfig {
	return ItemDecompose.GetMapItemId()
}
