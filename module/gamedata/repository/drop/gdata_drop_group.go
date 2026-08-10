// ============================================================================
// 本代码由xlsx工具自动生成，请勿手动修改
// Config: DropGroupConfig
// ============================================================================

package drop

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
//  接口定义
// ---------------------------------------------------------------------------

// IDropGroup 是 DropGroupConfig 的查询接口。
// 包级单例 DropGroup 实现该接口；亦可用于依赖注入、Mock 或测试替身。
type IDropGroup interface {
	// 基础查询
	GetHead() *protocol.DropGroupConfig
	GetAll() []*protocol.DropGroupConfig
	Count() int
	Range(fn func(*protocol.DropGroupConfig) bool)
	Find(fn func(*protocol.DropGroupConfig) bool) *protocol.DropGroupConfig
	Filter(fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig
	// 分组索引（一对多）
	GroupByGroupid(Groupid int32) []*protocol.DropGroupConfig
	GroupByGroupidFunc(Groupid int32, fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig
	GetMapGroupid() map[int32][]*protocol.DropGroupConfig
}

// ---------------------------------------------------------------------------
//  内部数据（不可变快照）
// ---------------------------------------------------------------------------

type DropGroupSnapshot struct {
	list         []*protocol.DropGroupConfig
	groupGroupid map[int32][]*protocol.DropGroupConfig
}

// ---------------------------------------------------------------------------
//  包级单例 + 注册加载
// ---------------------------------------------------------------------------

// DropGroup 是 DropGroupConfig 的包级单例，实现 IDropGroup。
// init() 自动注册到 gamedata，支持本地/远端加载与热更；测试可覆盖此变量做 Mock。
var DropGroup IDropGroup = &DropGroupImpl{}

var DropGroupPtr atomic.Pointer[DropGroupSnapshot]

type DropGroupImpl struct{}

func (c *DropGroupImpl) load() *DropGroupSnapshot {
	return DropGroupPtr.Load()
}

func init() {
	gamedata.Register("DropGroupConfig", parseDropGroup)
}

func parseDropGroup(buf string) error {
	data := &protocol.DropGroupConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	s := &DropGroupSnapshot{
		list:         data.Ary,
		groupGroupid: make(map[int32][]*protocol.DropGroupConfig),
	}

	for _, item := range data.Ary {
		s.groupGroupid[item.Groupid] = append(s.groupGroupid[item.Groupid], item)
	}

	DropGroupPtr.Store(s)
	return nil
}

// ---------------------------------------------------------------------------
//  基础查询
// ---------------------------------------------------------------------------

func (c *DropGroupImpl) GetHead() *protocol.DropGroupConfig {
	s := c.load()
	if s == nil || len(s.list) == 0 {
		return nil
	}
	return s.list[0]
}

func (c *DropGroupImpl) GetAll() []*protocol.DropGroupConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make([]*protocol.DropGroupConfig, len(s.list))
	copy(out, s.list)
	return out
}

func (c *DropGroupImpl) Count() int {
	s := c.load()
	if s == nil {
		return 0
	}
	return len(s.list)
}

func (c *DropGroupImpl) Range(fn func(*protocol.DropGroupConfig) bool) {
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

func (c *DropGroupImpl) Find(fn func(*protocol.DropGroupConfig) bool) *protocol.DropGroupConfig {
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

func (c *DropGroupImpl) Filter(fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	var out []*protocol.DropGroupConfig
	for _, item := range s.list {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

// 包级函数代理（兼容现有调用）
func GetDropGroupHead() *protocol.DropGroupConfig            { return DropGroup.GetHead() }
func GetDropGroupAll() []*protocol.DropGroupConfig           { return DropGroup.GetAll() }
func CountDropGroup() int                                    { return DropGroup.Count() }
func RangeDropGroup(fn func(*protocol.DropGroupConfig) bool) { DropGroup.Range(fn) }
func FindDropGroup(fn func(*protocol.DropGroupConfig) bool) *protocol.DropGroupConfig {
	return DropGroup.Find(fn)
}
func FilterDropGroup(fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig {
	return DropGroup.Filter(fn)
}

// ---------------------------------------------------------------------------
//  索引查询
// ---------------------------------------------------------------------------

func (c *DropGroupImpl) GroupByGroupid(Groupid int32) []*protocol.DropGroupConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	src := s.groupGroupid[Groupid]
	if len(src) == 0 {
		return nil
	}
	out := make([]*protocol.DropGroupConfig, len(src))
	copy(out, src)
	return out
}

// GroupByGroupidFunc 在分组内二次筛选：先按索引缩小到该 key 的记录，再回调过滤。
// 比全表 Filter 高效（O 组内数 vs O 全表数）。
func (c *DropGroupImpl) GroupByGroupidFunc(Groupid int32, fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	src := s.groupGroupid[Groupid]
	var out []*protocol.DropGroupConfig
	for _, item := range src {
		if fn(item) {
			out = append(out, item)
		}
	}
	return out
}

func (c *DropGroupImpl) GetMapGroupid() map[int32][]*protocol.DropGroupConfig {
	s := c.load()
	if s == nil {
		return nil
	}
	out := make(map[int32][]*protocol.DropGroupConfig, len(s.groupGroupid))
	for k, v := range s.groupGroupid {
		cp := make([]*protocol.DropGroupConfig, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// 包级函数代理
func GroupDropGroupByGroupid(Groupid int32) []*protocol.DropGroupConfig {
	return DropGroup.GroupByGroupid(Groupid)
}
func GroupDropGroupByGroupidFunc(Groupid int32, fn func(*protocol.DropGroupConfig) bool) []*protocol.DropGroupConfig {
	return DropGroup.GroupByGroupidFunc(Groupid, fn)
}
func GetDropGroupMapGroupid() map[int32][]*protocol.DropGroupConfig {
	return DropGroup.GetMapGroupid()
}
