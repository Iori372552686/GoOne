package room

import (
	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/gfunc"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func OnMainJoinRoom(c cmd_handler.IContext, req *g1_protocol.JoinRoomReq, myRole *role.Role) *g1_protocol.JoinRoomRsp {
	rsp := &g1_protocol.JoinRoomRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	if req.RoomId == 0 {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
		return rsp
	}

	req.ConnBusId = c.OriSrcBusId()
	err := c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_JOINROOM_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	if rsp.Ret.Code == g1_protocol.ErrorCode_ERR_NOT_EXIST_GAME_ROOM {
		myRole.ClearPlayRoomInfo()
		_ = myRole.FlushPending(c, false)
	}
	return rsp
}

func OnMainGetRoomList(c cmd_handler.IContext, req *g1_protocol.RoomListReq, myRole *role.Role) *g1_protocol.RoomListRsp {
	rsp := &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	if len(req.RoomIds) > 0 {
		//todo mget redis
		return rsp
	}

	routerID := gfunc.GetTexasRoomListIndex(c.Zone(), req.GameId, req.CoinType)
	rsp, err := roomcenterv1.NewRoomCenterInnerServiceClient().RoomListByRouter(c, routerID, req)
	if err != nil {
		if rsp == nil {
			rsp = &g1_protocol.RoomListRsp{Ret: &g1_protocol.Ret{}}
		}
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	return rsp
}

func OnMainQuickStart(c cmd_handler.IContext, req *g1_protocol.QuickStartReq, myRole *role.Role) *g1_protocol.QuickStartRsp {
	rsp := &g1_protocol.QuickStartRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_FAIL}}
	req.ConnBusId = c.OriSrcBusId()
	roomCenterClient := roomcenterv1.NewRoomCenterInnerServiceClient()

	for i := 0; i < 3; i++ {
		routerID := gfunc.GetTexasRoomListIndex(c.Zone(), req.GameId, req.CoinType)
		roomRsp, err := roomCenterClient.QuickStartByRouter(c, routerID, req)
		if err != nil {
			rsp.Ret.Msg = err.Error()
			logger.Errorf("quick start call RoomCenterSvr cur:%d | err: %s", i, rsp.Ret)
			continue
		}
		rsp = roomRsp

		if rsp.Ret.Code == g1_protocol.ErrorCode_ERR_OK && rsp.RoomInfo != nil {
			err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, rsp.RoomInfo.RoomId, g1_protocol.CMD_TEXAS_INNER_QUICK_START_REQ, req, rsp)
			if err != nil {
				rsp.Ret.Msg = err.Error()
				logger.Errorf("quick start call TexasGameSvr cur:%d | err: %v", i, rsp.Ret)
				continue
			}

			if rsp.Ret.Code == g1_protocol.ErrorCode_ERR_OK {
				myRole.AddPlayRoomID(rsp.RoomInfo.RoomId)
				_ = myRole.FlushPending(c, false)
				return rsp
			}
		}
	}

	return rsp
}

func OnMainExitRoom(c cmd_handler.IContext, req *g1_protocol.LeaveGameReq, myRole *role.Role) *g1_protocol.LeaveGameRsp {
	rsp := &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if req.RoomId == 0 {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_ARGV
		return rsp
	}

	err := c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_LEAVE_GAME_REQ, req, rsp)
	if err != nil {
		logger.Errorf("leave game call TexasGameSvr err: %v", err)
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	if rsp.Ret.Code == g1_protocol.ErrorCode_ERR_OK {
		myRole.RemovePlayRoomID(req.RoomId)
		_ = myRole.FlushPending(c, false)
	}

	return rsp
}
