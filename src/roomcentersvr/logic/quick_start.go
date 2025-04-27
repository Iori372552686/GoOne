package logic

import (
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_ai"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func OnCenterQuickStart(req *g1_protocol.QuickStartReq, roomMgr *texas_room.TexasRoomCenterMgr) *g1_protocol.QuickStartRsp {
	rsp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	texas := roomMgr.GetTexasObj(int32(req.Stage))
	if texas.RoomsMap == nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_NOT_EXIST_GAME_ROOM
		return rsp
	}

	for _, room := range texas.RoomsMap {
		if room.Base.MaxPlayer < room.Base.CurPlayerNum {
			room.Base.CurPlayerNum++
			rsp.RoomInfo = room.Base

			texas.Save()
			return rsp
		}
	}

	// 如果没有空余的房间，创建一个新的房间
	base, err := room_ai.OnAiCreatRoom(req.GameId, int32(req.CoinType), int32(req.Stage))
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_TEXAS_SEAT_NOT_FOUND
	}

	rsp.RoomInfo = base
	return rsp
}
