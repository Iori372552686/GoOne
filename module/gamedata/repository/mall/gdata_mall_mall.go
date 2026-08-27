// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: MallConfig
// ============================================================================

package mall

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

// IMall 是 MallConfig 的查询接口。
// 包级单例 Mall 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IMall interface {
	// 基础查询
	GetHead() *protocol.MallConfig
	GetAll() []*protocol.MallConfig
	Count() int
	Range(fn func(*protocol.MallConfig) bool)
	Find(fn func(*protocol.MallConfig) bool) *protocol.MallConfig
	Filter(fn func(*protocol.MallConfig) bool) []*protocol.MallConfig
	// 主键索引（唯一）
	GetById(Id int32) *protocol.MallConfig
	MustGetById(Id int32) *protocol.MallConfig
	HasById(Id int32) bool
	GetMapId() map[int32]*protocol.MallConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type MallSnapshot struct {
	list  []*protocol.MallConfig
	mapId map[int32]*protocol.MallConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// Mall 是 MallConfig 的包级单例，实现 IMall。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var Mall IMall = &MallImpl{}

var MallPtr atomic.Pointer[MallSnapshot]

type MallImpl struct{}

func (c *MallImpl) load() *MallSnapshot {
	return MallPtr.Load()
}

func init() {
	gamedata.Register("MallConfig", parseMall)
}

func parseMall(buf string) error {
	data := &protocol.MallConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &MallSnapshot{
		list:  data.Ary,
		mapId: make(map[int32]*protocol.MallConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapId[item.Id]; exists {
			return fmt.Errorf("MallConfig 重复主键 Id=%v", item.Id)
		}
		s.mapId[item.Id] = item
	}

	MallPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *MallImpl) GetHead() *protocol.MallConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *MallImpl) GetAll() []*protocol.MallConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.MallConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *MallImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *MallImpl) Range(fn func(*protocol.MallConfig) bool) {
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

func (c *MallImpl) Find(fn func(*protocol.MallConfig) bool) *protocol.MallConfig {
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

func (c *MallImpl) Filter(fn func(*protocol.MallConfig) bool) []*protocol.MallConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.MallConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetMallHead() *protocol.MallConfig            { return Mall.GetHead() }
func GetMallAll() []*protocol.MallConfig           { return Mall.GetAll() }
func CountMall() int                               { return Mall.Count() }
func RangeMall(fn func(*protocol.MallConfig) bool) { Mall.Range(fn) }
func FindMall(fn func(*protocol.MallConfig) bool) *protocol.MallConfig {
	return Mall.Find(fn)
}
func FilterMall(fn func(*protocol.MallConfig) bool) []*protocol.MallConfig {
	return Mall.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *MallImpl) GetById(Id int32) *protocol.MallConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapId[Id]
}

// MustGetById 未命中时 panic，用于配置必须存在的确定性场景
func (c *MallImpl) MustGetById(Id int32) *protocol.MallConfig {
	v := c.GetById(Id)
	if v == nil {
		panic(fmt.Sprintf("MallConfig 主键 Id=%v 不存在", Id))
	}
	return v
}

func (c *MallImpl) HasById(Id int32) bool {
	return c.GetById(Id) != nil
}

func (c *MallImpl) GetMapId() map[int32]*protocol.MallConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.MallConfig, len(s.mapId))
	for k, v := range s.mapId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetMallById(Id int32) *protocol.MallConfig {
	return Mall.GetById(Id)
}
func MustGetMallById(Id int32) *protocol.MallConfig {
	return Mall.MustGetById(Id)
}
func HasMallById(Id int32) bool {
	return Mall.HasById(Id)
}
func GetMallMapId() map[int32]*protocol.MallConfig {
	return Mall.GetMapId()
}
