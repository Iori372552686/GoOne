// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: TexasConfig
// ============================================================================

package texas

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

// ITexas 是 TexasConfig 的查询接口。
// 包级单例 Texas 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type ITexas interface {
	// 基础查询
	GetHead() *protocol.TexasConfig
	GetAll() []*protocol.TexasConfig
	Count() int
	Range(fn func(*protocol.TexasConfig) bool)
	Find(fn func(*protocol.TexasConfig) bool) *protocol.TexasConfig
	Filter(fn func(*protocol.TexasConfig) bool) []*protocol.TexasConfig
	// 主键索引（唯一）
	GetByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig
	MustGetByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig
	HasByRoomStageCoinType(RoomStage int32, CoinType int32) bool
	GetMapRoomStageCoinType() map[gamedata.Index2[int32, int32]]*protocol.TexasConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type TexasSnapshot struct {
	list                 []*protocol.TexasConfig
	mapRoomStageCoinType map[gamedata.Index2[int32, int32]]*protocol.TexasConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// Texas 是 TexasConfig 的包级单例，实现 ITexas。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var Texas ITexas = &TexasImpl{}

var TexasPtr atomic.Pointer[TexasSnapshot]

type TexasImpl struct{}

func (c *TexasImpl) load() *TexasSnapshot {
	return TexasPtr.Load()
}

func init() {
	gamedata.Register("TexasConfig", parseTexas)
}

func parseTexas(buf string) error {
	data := &protocol.TexasConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &TexasSnapshot{
		list:                 data.Ary,
		mapRoomStageCoinType: make(map[gamedata.Index2[int32, int32]]*protocol.TexasConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		_keyRoomStageCoinType := gamedata.Index2[int32, int32]{T2: item.RoomStage, T1: item.CoinType}
		if _, exists := s.mapRoomStageCoinType[_keyRoomStageCoinType]; exists {
			return fmt.Errorf("TexasConfig 重复复合主键 RoomStageCoinType")
		}
		s.mapRoomStageCoinType[_keyRoomStageCoinType] = item
	}

	TexasPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *TexasImpl) GetHead() *protocol.TexasConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *TexasImpl) GetAll() []*protocol.TexasConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.TexasConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *TexasImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *TexasImpl) Range(fn func(*protocol.TexasConfig) bool) {
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

func (c *TexasImpl) Find(fn func(*protocol.TexasConfig) bool) *protocol.TexasConfig {
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

func (c *TexasImpl) Filter(fn func(*protocol.TexasConfig) bool) []*protocol.TexasConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.TexasConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetTexasHead() *protocol.TexasConfig            { return Texas.GetHead() }
func GetTexasAll() []*protocol.TexasConfig           { return Texas.GetAll() }
func CountTexas() int                                { return Texas.Count() }
func RangeTexas(fn func(*protocol.TexasConfig) bool) { Texas.Range(fn) }
func FindTexas(fn func(*protocol.TexasConfig) bool) *protocol.TexasConfig {
	return Texas.Find(fn)
}
func FilterTexas(fn func(*protocol.TexasConfig) bool) []*protocol.TexasConfig {
	return Texas.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *TexasImpl) GetByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapRoomStageCoinType[gamedata.Index2[int32, int32]{T2: RoomStage, T1: CoinType}]
}

// MustGetByRoomStageCoinType 未命中时 panic，用于配置必须存在的确定性场景
func (c *TexasImpl) MustGetByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig {
	v := c.GetByRoomStageCoinType(RoomStage, CoinType)
	if v == nil {
		panic(fmt.Sprintf("TexasConfig 主键 RoomStageCoinType=%v, %v 不存在", RoomStage, CoinType))
	}
	return v
}

func (c *TexasImpl) HasByRoomStageCoinType(RoomStage int32, CoinType int32) bool {
	return c.GetByRoomStageCoinType(RoomStage, CoinType) != nil
}

func (c *TexasImpl) GetMapRoomStageCoinType() map[gamedata.Index2[int32, int32]]*protocol.TexasConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[gamedata.Index2[int32, int32]]*protocol.TexasConfig, len(s.mapRoomStageCoinType))
	for k, v := range s.mapRoomStageCoinType {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetTexasByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig {
	return Texas.GetByRoomStageCoinType(RoomStage, CoinType)
}
func MustGetTexasByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig {
	return Texas.MustGetByRoomStageCoinType(RoomStage, CoinType)
}
func HasTexasByRoomStageCoinType(RoomStage int32, CoinType int32) bool {
	return Texas.HasByRoomStageCoinType(RoomStage, CoinType)
}
func GetTexasMapRoomStageCoinType() map[gamedata.Index2[int32, int32]]*protocol.TexasConfig {
	return Texas.GetMapRoomStageCoinType()
}
