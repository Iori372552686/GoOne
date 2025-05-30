package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	logger.Infof("register transaction commands")

	globals.TransMgr.RegisterCmd(g1_protocol.CMD_ROOM_CENTER_INNER_ROOM_LIST_REQ, NewZoneAdapter(InnerGetRoomList))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_ROOM_CENTER_INNER_UPDATE_ROOM_INFO_REQ, NewZoneAdapter(InnerUpdateRoomInfo))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_ROOM_CENTER_INNER_DEL_ROOM_INFO_REQ, NewZoneAdapter(InnerDelRoomInfo))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_ROOM_CENTER_INNER_QUICK_START_REQ, NewZoneAdapter(InnerQuickStart))

	//inner
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_ROOM_CENTER_INNER_TICK_REQ, NewZoneAdapter(InnerTick))
}
