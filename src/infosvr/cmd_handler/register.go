package cmd_handler

import (
	g1_protocol `GoOne/protobuf/protocol`
	`GoOne/src/infosvr/globals`

	"github.com/golang/glog"

)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	glog.Infof("register transaction commands")
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_INFO_GET_BRIEF_INFO_REQ), new(GetBriefInfo))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_INFO_GET_ICON_DESC_REQ), new(GetIconDesc))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_INFO_INNER_SET_BRIEF_INFO_REQ), new(SetBriefInfo))
}
