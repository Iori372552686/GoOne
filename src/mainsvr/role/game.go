package role

import (
	util "github.com/Iori372552686/GoOne/lib/util/slices"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

func (r *Role) AddPlayRoomID(roomId uint64) pb.ErrorCode {
	info := r.PbRole.GameInfo

	if info.PlayRoomIds == nil {
		info.PlayRoomIds = make([]uint64, 0)
	}

	for _, v := range info.PlayRoomIds {
		if v == roomId {
			return pb.ErrorCode_ERR_OK
		}
	}

	info.PlayRoomIds = util.InsertAtTail(info.PlayRoomIds, roomId, 3)
	return pb.ErrorCode_ERR_OK
}

func (r *Role) RemovePlayRoomID(roomId uint64) pb.ErrorCode {
	info := r.PbRole.GameInfo

	if info.PlayRoomIds != nil {
		info.PlayRoomIds, _ = util.Remove(info.PlayRoomIds, roomId)
	}

	return pb.ErrorCode_ERR_OK
}

func (r *Role) ClearPlayRoomInfo() pb.ErrorCode {
	info := r.PbRole.GameInfo

	info.PlayRoomIds = make([]uint64, 0)
	return pb.ErrorCode_ERR_OK
}
