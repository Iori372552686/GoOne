package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

type AdapterZoneMgrFunc func(c cmd_handler.IContext, data []byte, roomMgr *texas_room.TexasRoomCenterMgr) g1_protocol.ErrorCode

// logAdapter is a adapter for game log command
type logAdapter struct {
	Cmd AdapterZoneMgrFunc
}

func NewZoneAdapter(rCmd AdapterZoneMgrFunc) cmd_handler.CmdHandlerFunc {
	a := new(logAdapter)
	a.Cmd = rCmd
	return a.ProcessCmd
}

func (impl *logAdapter) ProcessCmd(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	ins := globals.RoomListMgr.GetRoomMgrObj(c.Rid())
	if ins == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	return impl.Cmd(c, data, ins)
}
