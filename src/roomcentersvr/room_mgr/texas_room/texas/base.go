package texas

import (
	"errors"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

type TexasRoom struct {
	*pb.DBTexasRoomCenterInfo //TexasRoomInfo pb

	//private
	upTime   int64
	isChange bool
}

func NewTexasRoomObj(index uint64, stage int32) *TexasRoom {
	ins := &TexasRoom{}
	ins.init(index, stage)
	return ins
}

func (impl *TexasRoom) init(index uint64, stage int32) {
	impl.DBTexasRoomCenterInfo = &pb.DBTexasRoomCenterInfo{
		Index:    index,
		Stage:    pb.RoomStage(stage),
		RoomsMap: make(map[uint64]*pb.RoomShowInfo),
	}
}

func (impl *TexasRoom) Get() *pb.DBTexasRoomCenterInfo {
	return impl.DBTexasRoomCenterInfo
}

func (impl *TexasRoom) Save() {
	impl.isChange = true
}

// MarkSaved 标记房间数据已持久化（清 dirty）。
// 由 data_proc.go 的 saveRoomDataToDB / FlushAllRoomsToDB 在写入成功后调用。
func (impl *TexasRoom) MarkSaved() {
	impl.isChange = false
}

func (impl *TexasRoom) CheckChange() bool {
	return impl.isChange
}

func (impl *TexasRoom) Update() (err error) {
	// 持久化逻辑已迁移至 texas_room/data_proc.go（saveRoomDataToDB）。
	// 此处仅保留方法签名以兼容历史调用，dirty 由 MarkSaved 管理。
	impl.isChange = false
	return nil
}

func (impl *TexasRoom) Set(data *pb.DBTexasRoomCenterInfo) error {
	if impl == nil || data == nil {
		return errors.New("param error")
	}

	impl.DBTexasRoomCenterInfo = data
	impl.upTime = datetime.NowMs()
	impl.isChange = true
	return nil
}
