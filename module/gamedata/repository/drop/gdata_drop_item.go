// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: DropItemConfig
// ============================================================================

package drop

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

// IDropItem 是 DropItemConfig 的查询接口。
// 包级单例 DropItem 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IDropItem interface {
	// 基础查询
	GetHead() *protocol.DropItemConfig
	GetAll() []*protocol.DropItemConfig
	Count() int
	Range(fn func(*protocol.DropItemConfig) bool)
	Find(fn func(*protocol.DropItemConfig) bool) *protocol.DropItemConfig
	Filter(fn func(*protocol.DropItemConfig) bool) []*protocol.DropItemConfig
	// 主键索引（唯一）
	GetByDropItemId(DropItemId int32) *protocol.DropItemConfig
	MustGetByDropItemId(DropItemId int32) *protocol.DropItemConfig
	HasByDropItemId(DropItemId int32) bool
	GetMapDropItemId() map[int32]*protocol.DropItemConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type DropItemSnapshot struct {
	list          []*protocol.DropItemConfig
	mapDropItemId map[int32]*protocol.DropItemConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// DropItem 是 DropItemConfig 的包级单例，实现 IDropItem。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var DropItem IDropItem = &DropItemImpl{}

var DropItemPtr atomic.Pointer[DropItemSnapshot]

type DropItemImpl struct{}

func (c *DropItemImpl) load() *DropItemSnapshot {
	return DropItemPtr.Load()
}

func init() {
	gamedata.Register("DropItemConfig", parseDropItem)
}

func parseDropItem(buf string) error {
	data := &protocol.DropItemConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &DropItemSnapshot{
		list:          data.Ary,
		mapDropItemId: make(map[int32]*protocol.DropItemConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapDropItemId[item.DropItemId]; exists {
			return fmt.Errorf("DropItemConfig 重复主键 DropItemId=%v", item.DropItemId)
		}
		s.mapDropItemId[item.DropItemId] = item
	}

	DropItemPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *DropItemImpl) GetHead() *protocol.DropItemConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *DropItemImpl) GetAll() []*protocol.DropItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.DropItemConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *DropItemImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *DropItemImpl) Range(fn func(*protocol.DropItemConfig) bool) {
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

func (c *DropItemImpl) Find(fn func(*protocol.DropItemConfig) bool) *protocol.DropItemConfig {
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

func (c *DropItemImpl) Filter(fn func(*protocol.DropItemConfig) bool) []*protocol.DropItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.DropItemConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetDropItemHead() *protocol.DropItemConfig            { return DropItem.GetHead() }
func GetDropItemAll() []*protocol.DropItemConfig           { return DropItem.GetAll() }
func CountDropItem() int                                   { return DropItem.Count() }
func RangeDropItem(fn func(*protocol.DropItemConfig) bool) { DropItem.Range(fn) }
func FindDropItem(fn func(*protocol.DropItemConfig) bool) *protocol.DropItemConfig {
	return DropItem.Find(fn)
}
func FilterDropItem(fn func(*protocol.DropItemConfig) bool) []*protocol.DropItemConfig {
	return DropItem.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *DropItemImpl) GetByDropItemId(DropItemId int32) *protocol.DropItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapDropItemId[DropItemId]
}

// MustGetByDropItemId 未命中时 panic，用于配置必须存在的确定性场景
func (c *DropItemImpl) MustGetByDropItemId(DropItemId int32) *protocol.DropItemConfig {
	v := c.GetByDropItemId(DropItemId)
	if v == nil {
		panic(fmt.Sprintf("DropItemConfig 主键 DropItemId=%v 不存在", DropItemId))
	}
	return v
}

func (c *DropItemImpl) HasByDropItemId(DropItemId int32) bool {
	return c.GetByDropItemId(DropItemId) != nil
}

func (c *DropItemImpl) GetMapDropItemId() map[int32]*protocol.DropItemConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.DropItemConfig, len(s.mapDropItemId))
	for k, v := range s.mapDropItemId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetDropItemByDropItemId(DropItemId int32) *protocol.DropItemConfig {
	return DropItem.GetByDropItemId(DropItemId)
}
func MustGetDropItemByDropItemId(DropItemId int32) *protocol.DropItemConfig {
	return DropItem.MustGetByDropItemId(DropItemId)
}
func HasDropItemByDropItemId(DropItemId int32) bool {
	return DropItem.HasByDropItemId(DropItemId)
}
func GetDropItemMapDropItemId() map[int32]*protocol.DropItemConfig {
	return DropItem.GetMapDropItemId()
}
