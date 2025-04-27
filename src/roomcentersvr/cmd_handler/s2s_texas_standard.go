package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/logic"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// CMD_ROOM_CENTER_INNER_TICK_REQ
func InnerTick(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	req := &g1_protocol.InnerTickReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	roomMgr.Tick(req.NowMs)
	return g1_protocol.ErrorCode_ERR_OK
}

// CMD_ROOM_CENTER_INNER_QUICK_START_REQ
func InnerQuickStart(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	req := &g1_protocol.QuickStartReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	c.SendMsgBack(logic.OnCenterQuickStart(req, roomMgr))
	return g1_protocol.ErrorCode_ERR_OK
}

// CMD_ROOM_CENTER_INNER_ROOM_LIST_REQ
func InnerGetRoomList(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	req := &g1_protocol.RoomListReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	c.SendMsgBack(logic.OnCenterRoomList(req, roomMgr))
	return g1_protocol.ErrorCode_ERR_OK
}

// CMD_ROOM_CENTER_INNER_UPDATE_ROOM_INFO_REQ
func InnerUpdateRoomInfo(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	req := &g1_protocol.RoomShowInfo{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	return logic.OnUpdateTexasRoomInfo(req, roomMgr)
}

// CMD_ROOM_CENTER_INNER_DEL_ROOM_INFO_REQ
func InnerDelRoomInfo(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode {
	req := &g1_protocol.RoomShowInfo{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	return logic.OnDelTexasRoomInfo(req, roomMgr)
}
