package texas

import (
	"errors"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	pb "github.com/Iori372552686/g1_common/protocol"
)

// TexasRoom 单个 stage（场次级别）的房间表。
//
// 并发约定：
//   - 数据本体是 pb.DBTexasRoomCenterInfo（含 RoomsMap）；对 RoomsMap 的结构变更
//     （增删房间）与房间字段的原地修改（CurPlayerNum 等）必须在所属
//     TexasRoomCenterMgr 的锁内进行，调用方不得绕过封装直接触碰裸 map。
//   - isChange 使用 atomic.Bool 做双保险：即使个别路径漏持锁调用 Save/MarkSaved，
//     脏标志本身也不会产生数据竞争（原 bool 字段在"持久化读"与"业务写"并发时是 race）。
type TexasRoom struct {
	*pb.DBTexasRoomCenterInfo //TexasRoomInfo pb

	//private
	upTime   int64
	isChange atomic.Bool // 快照脏标志：true 表示有变更等待周期持久化
}

// NewTexasRoomObj 创建一个空的本 stage 房间表（懒创建入口）。
func NewTexasRoomObj(index uint64, stage int32) *TexasRoom {
	ins := &TexasRoom{}
	ins.init(index, stage)
	return ins
}

// init 初始化内部 pb 结构（仅构造时调用一次，之后不再替换 map 头）。
func (impl *TexasRoom) init(index uint64, stage int32) {
	impl.DBTexasRoomCenterInfo = &pb.DBTexasRoomCenterInfo{
		Index:    index,
		Stage:    pb.RoomStage(stage),
		RoomsMap: make(map[uint64]*pb.RoomShowInfo),
	}
}

// Get 取底层 pb 数据（持久化序列化用；返回的内部 map 仍受锁约定约束）。
func (impl *TexasRoom) Get() *pb.DBTexasRoomCenterInfo {
	return impl.DBTexasRoomCenterInfo
}

// Save 标记房间表有变更，等待周期持久化（TickPersist 10s 节拍）落盘。
func (impl *TexasRoom) Save() {
	impl.isChange.Store(true)
}

// MarkSaved 标记快照已持久化（清 dirty）。
// 由 data_proc.go 的 SaveRoomDataToDB / FlushAllRoomsToDB 在写入成功后调用。
func (impl *TexasRoom) MarkSaved() {
	impl.isChange.Store(false)
}

// CheckChange 是否存在未落盘的变更。
func (impl *TexasRoom) CheckChange() bool {
	return impl.isChange.Load()
}

// Set 整表替换（快照恢复用）。必须在 TexasRoomCenterMgr 写锁内调用，
// 否则会丢弃并发插入的房间。
func (impl *TexasRoom) Set(data *pb.DBTexasRoomCenterInfo) error {
	if impl == nil || data == nil {
		return errors.New("param error")
	}

	impl.DBTexasRoomCenterInfo = data
	impl.upTime = datetime.NowMs()
	impl.isChange.Store(true)
	return nil
}
