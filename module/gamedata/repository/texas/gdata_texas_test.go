// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: TexasTestConfig
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

// ITexasTest 是 TexasTestConfig 的查询接口。
// 包级单例 TexasTest 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type ITexasTest interface {
	// 基础查询
	GetHead() *protocol.TexasTestConfig
	GetAll() []*protocol.TexasTestConfig
	Count() int
	Range(fn func(*protocol.TexasTestConfig) bool)
	Find(fn func(*protocol.TexasTestConfig) bool) *protocol.TexasTestConfig
	Filter(fn func(*protocol.TexasTestConfig) bool) []*protocol.TexasTestConfig
	// 主键索引（唯一）
	GetByRound(Round uint32) *protocol.TexasTestConfig
	MustGetByRound(Round uint32) *protocol.TexasTestConfig
	HasByRound(Round uint32) bool
	GetMapRound() map[uint32]*protocol.TexasTestConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type TexasTestSnapshot struct {
	list     []*protocol.TexasTestConfig
	mapRound map[uint32]*protocol.TexasTestConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// TexasTest 是 TexasTestConfig 的包级单例，实现 ITexasTest。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var TexasTest ITexasTest = &TexasTestImpl{}

var TexasTestPtr atomic.Pointer[TexasTestSnapshot]

type TexasTestImpl struct{}

func (c *TexasTestImpl) load() *TexasTestSnapshot {
	return TexasTestPtr.Load()
}

func init() {
	gamedata.Register("TexasTestConfig", parseTexasTest)
}

func parseTexasTest(buf string) error {
	data := &protocol.TexasTestConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &TexasTestSnapshot{
		list:     data.Ary,
		mapRound: make(map[uint32]*protocol.TexasTestConfig, len(data.Ary)),
	}

	for _, item := range data.Ary {
		// 重复主键检测：配置数据正确性兜底
		if _, exists := s.mapRound[item.Round]; exists {
			return fmt.Errorf("TexasTestConfig 重复主键 Round=%v", item.Round)
		}
		s.mapRound[item.Round] = item
	}

	TexasTestPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *TexasTestImpl) GetHead() *protocol.TexasTestConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *TexasTestImpl) GetAll() []*protocol.TexasTestConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.TexasTestConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *TexasTestImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *TexasTestImpl) Range(fn func(*protocol.TexasTestConfig) bool) {
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

func (c *TexasTestImpl) Find(fn func(*protocol.TexasTestConfig) bool) *protocol.TexasTestConfig {
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

func (c *TexasTestImpl) Filter(fn func(*protocol.TexasTestConfig) bool) []*protocol.TexasTestConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.TexasTestConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetTexasTestHead() *protocol.TexasTestConfig            { return TexasTest.GetHead() }
func GetTexasTestAll() []*protocol.TexasTestConfig           { return TexasTest.GetAll() }
func CountTexasTest() int                                    { return TexasTest.Count() }
func RangeTexasTest(fn func(*protocol.TexasTestConfig) bool) { TexasTest.Range(fn) }
func FindTexasTest(fn func(*protocol.TexasTestConfig) bool) *protocol.TexasTestConfig {
	return TexasTest.Find(fn)
}
func FilterTexasTest(fn func(*protocol.TexasTestConfig) bool) []*protocol.TexasTestConfig {
	return TexasTest.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *TexasTestImpl) GetByRound(Round uint32) *protocol.TexasTestConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	return s.mapRound[Round]
}

// MustGetByRound 未命中时 panic，用于配置必须存在的确定性场景
func (c *TexasTestImpl) MustGetByRound(Round uint32) *protocol.TexasTestConfig {
	v := c.GetByRound(Round)
	if v == nil {
		panic(fmt.Sprintf("TexasTestConfig 主键 Round=%v 不存在", Round))
	}
	return v
}

func (c *TexasTestImpl) HasByRound(Round uint32) bool {
	return c.GetByRound(Round) != nil
}

func (c *TexasTestImpl) GetMapRound() map[uint32]*protocol.TexasTestConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[uint32]*protocol.TexasTestConfig, len(s.mapRound))
	for k, v := range s.mapRound {
		out[k] = v
	}
	return out
}

// 包级函数代理
func GetTexasTestByRound(Round uint32) *protocol.TexasTestConfig {
	return TexasTest.GetByRound(Round)
}
func MustGetTexasTestByRound(Round uint32) *protocol.TexasTestConfig {
	return TexasTest.MustGetByRound(Round)
}
func HasTexasTestByRound(Round uint32) bool {
	return TexasTest.HasByRound(Round)
}
func GetTexasTestMapRound() map[uint32]*protocol.TexasTestConfig {
	return TexasTest.GetMapRound()
}
