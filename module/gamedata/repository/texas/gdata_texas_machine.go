// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: TexasMachineConfig
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

// ITexasMachine 是 TexasMachineConfig 的查询接口。
// 包级单例 TexasMachine 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type ITexasMachine interface {
	// 基础查询
	GetHead() *protocol.TexasMachineConfig
	GetAll() []*protocol.TexasMachineConfig
	Count() int
	Range(fn func(*protocol.TexasMachineConfig) bool)
	Find(fn func(*protocol.TexasMachineConfig) bool) *protocol.TexasMachineConfig
	Filter(fn func(*protocol.TexasMachineConfig) bool) []*protocol.TexasMachineConfig
	// 主键索引（唯一）
	GetByGameId(GameId int32) *protocol.TexasMachineConfig
	MustGetByGameId(GameId int32) *protocol.TexasMachineConfig
	HasByGameId(GameId int32) bool
	GetMapGameId() map[int32]*protocol.TexasMachineConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type TexasMachineSnapshot struct {
	list      []*protocol.TexasMachineConfig
	mapGameId map[int32]*protocol.TexasMachineConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// TexasMachine 是 TexasMachineConfig 的包级单例，实现 ITexasMachine。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var TexasMachine ITexasMachine = &TexasMachineImpl{}

var TexasMachinePtr atomic.Pointer[TexasMachineSnapshot]

type TexasMachineImpl struct{}

func (c *TexasMachineImpl) load() *TexasMachineSnapshot {
	return TexasMachinePtr.Load()
}

func init() {
	gamedata.Register("TexasMachineConfig", parseTexasMachine)
}

func parseTexasMachine(buf string) error {
	data := &protocol.TexasMachineConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &TexasMachineSnapshot{
		list:      data.Ary,
		mapGameId: make(map[int32]*protocol.TexasMachineConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapGameId[item.GameId]; exists {
			return fmt.Errorf("TexasMachineConfig 重复主键 GameId=%v", item.GameId)
		}
		s.mapGameId[item.GameId] = item
	}

	TexasMachinePtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *TexasMachineImpl) GetHead() *protocol.TexasMachineConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *TexasMachineImpl) GetAll() []*protocol.TexasMachineConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.TexasMachineConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *TexasMachineImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *TexasMachineImpl) Range(fn func(*protocol.TexasMachineConfig) bool) {
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

func (c *TexasMachineImpl) Find(fn func(*protocol.TexasMachineConfig) bool) *protocol.TexasMachineConfig {
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

func (c *TexasMachineImpl) Filter(fn func(*protocol.TexasMachineConfig) bool) []*protocol.TexasMachineConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.TexasMachineConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetTexasMachineHead() *protocol.TexasMachineConfig            { return TexasMachine.GetHead() }
func GetTexasMachineAll() []*protocol.TexasMachineConfig           { return TexasMachine.GetAll() }
func CountTexasMachine() int                                       { return TexasMachine.Count() }
func RangeTexasMachine(fn func(*protocol.TexasMachineConfig) bool) { TexasMachine.Range(fn) }
func FindTexasMachine(fn func(*protocol.TexasMachineConfig) bool) *protocol.TexasMachineConfig {
	return TexasMachine.Find(fn)
}
func FilterTexasMachine(fn func(*protocol.TexasMachineConfig) bool) []*protocol.TexasMachineConfig {
	return TexasMachine.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *TexasMachineImpl) GetByGameId(GameId int32) *protocol.TexasMachineConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapGameId[GameId]
}

// MustGetByGameId 未命中时 panic，用于配置必须存在的确定性场景
func (c *TexasMachineImpl) MustGetByGameId(GameId int32) *protocol.TexasMachineConfig {
	v := c.GetByGameId(GameId)
	if v == nil {
		panic(fmt.Sprintf("TexasMachineConfig 主键 GameId=%v 不存在", GameId))
	}
	return v
}

func (c *TexasMachineImpl) HasByGameId(GameId int32) bool {
	return c.GetByGameId(GameId) != nil
}

func (c *TexasMachineImpl) GetMapGameId() map[int32]*protocol.TexasMachineConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32]*protocol.TexasMachineConfig, len(s.mapGameId))
	for k, v := range s.mapGameId {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetTexasMachineByGameId(GameId int32) *protocol.TexasMachineConfig {
	return TexasMachine.GetByGameId(GameId)
}
func MustGetTexasMachineByGameId(GameId int32) *protocol.TexasMachineConfig {
	return TexasMachine.MustGetByGameId(GameId)
}
func HasTexasMachineByGameId(GameId int32) bool {
	return TexasMachine.HasByGameId(GameId)
}
func GetTexasMachineMapGameId() map[int32]*protocol.TexasMachineConfig {
	return TexasMachine.GetMapGameId()
}
