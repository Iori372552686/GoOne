package logic

import (
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func OnCenterRoomList(req *g1_protocol.RoomListReq, roomMgr *texas_room.TexasRoomCenterMgr) *g1_protocol.RoomListRsp {
	rsp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	rsp.Stage = req.Stage
	rsp.GameId = req.GameId
	rsp.PageIndex = req.PageIndex
	rsp.PageSize = req.PageSize
	rsp.CoinType = req.CoinType
	var rooms []*g1_protocol.RoomShowInfo

	if req.PageIndex < 1 || req.PageSize < 1 || req.PageSize > 500 {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
		return rsp
	}

	// 根据游戏ID和阶段获取房间列表
	if req.Stage == g1_protocol.RoomStage_Stage_ALL {
		cnt := 0
		for _, texas := range roomMgr.TexasMap {
			if texas.RoomsMap != nil {
				cnt += len(texas.RoomsMap)
			}
		}

		if cnt == 0 {
			return rsp
		}

		rooms = make([]*g1_protocol.RoomShowInfo, 0, cnt)
		for _, texas := range roomMgr.TexasMap {
			if texas.RoomsMap != nil {
				for _, room := range texas.RoomsMap {
					if room.Base != nil {
						rooms = append(rooms, room)
					}
				}
			}
		}
	} else {
		rMap := roomMgr.GetTexasObj(int32(req.Stage))
		if rMap == nil || len(rMap.RoomsMap) == 0 {
			return rsp
		}

		rooms = make([]*g1_protocol.RoomShowInfo, 0, len(rMap.RoomsMap))
		for _, room := range rMap.RoomsMap {
			if room.Base != nil {
				rooms = append(rooms, room)
			}
		}
	}

	// sort chose
	if req.SortType != g1_protocol.RoomSortType_SORT_TYPE_NONE {
		var less func(i, j int) bool

		switch req.SortType {
		case g1_protocol.RoomSortType_SORT_TYPE_ID:
			less = func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId }
		case g1_protocol.RoomSortType_SORT_TYPE_PLAYER:
			less = func(i, j int) bool { return rooms[i].Base.CurPlayerNum > rooms[j].Base.CurPlayerNum }
		case g1_protocol.RoomSortType_SORT_TYPE_TIME:
			less = func(i, j int) bool { return rooms[i].Base.EndTime > rooms[j].Base.EndTime }
		default:
			rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
			return rsp
		}

		parallelSort(rooms, less)
	}

	// calc page
	total := uint32(len(rooms))
	start := (req.PageIndex - 1) * req.PageSize
	end := req.PageIndex * req.PageSize

	// safe
	if start >= total {
		start, end = 0, 0
	} else if end > total {
		end = total
	}

	if start < end {
		rsp.RoomList = rooms[start:end:end] // 带容量限制防止后续误修改
	}
	rsp.TotalCount = total
	return rsp
}

func OnUpdateTexasRoomInfo(req *g1_protocol.RoomShowInfo, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	if req == nil || req.Base == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	texas := roomMgr.GetTexasObj(int32(req.Base.Stage))
	if texas.RoomsMap == nil {
		texas.RoomsMap = make(map[uint64]*g1_protocol.RoomShowInfo)
	}

	texas.RoomsMap[req.Base.RoomId] = req
	texas.Save()
	return g1_protocol.ErrorCode_ERR_OK
}

func OnDelTexasRoomInfo(req *g1_protocol.RoomShowInfo, mgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	if req == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	texas := mgr.GetTexasObj(int32(req.Base.Stage))
	if texas.RoomsMap != nil {
		delete(texas.RoomsMap, req.Base.RoomId)
		texas.Save()
	}

	return g1_protocol.ErrorCode_ERR_OK
}
