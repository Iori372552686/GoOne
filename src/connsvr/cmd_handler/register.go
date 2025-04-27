package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	logger.Infof("register transaction commands")
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_CONN_BROADCAST_REQ, Broadcast)
}
