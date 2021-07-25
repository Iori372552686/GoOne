package cmd_handler

import (
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/connsvr/globals"
	"github.com/golang/glog"

)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	glog.Infof("register transaction commands")
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_CONN_BROADCAST_REQ), new(Broadcast))
}
