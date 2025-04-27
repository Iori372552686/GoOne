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

func (impl *TexasRoom) CheckChange() bool {
	return impl.isChange
}

func (impl *TexasRoom) Update() (err error) {
	data := impl.Get()
	if data == nil {
		return
	}

	//err = global.GlobalDB.SetDBGuildInstanceData(ctx, data)
	if err != nil {
		return
	}

	impl.isChange = false
	return
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
