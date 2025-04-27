package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	logger.Infof("register transaction commands")

	globals.TransMgr.RegisterCmd(g1_protocol.CMD_INFO_GET_BRIEF_INFO_REQ, GetBriefInfo)
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_INFO_GET_ICON_DESC_REQ, GetIconDesc)
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_INFO_INNER_SET_BRIEF_INFO_REQ, SetBriefInfo)
}
