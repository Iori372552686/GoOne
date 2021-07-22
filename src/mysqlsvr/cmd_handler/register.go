package cmd_handler

import (
	g1_protocol `bian/src/bian_newFrame/protobuf/protocol`
	`bian/src/bian_newFrame/src/mysqlsvr/globals`
	`bian/src/common/logger`
)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	logger.Infof("register transaction commands")
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MYSQL_INNER_UPDATE_ROLE_INFO_REQ), new(UpdateRoleInfo))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MYSQL_INNER_SEARCH_ROLE_REQ), new(SearchRole))
	//globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MYSQL_INNER_SEARCH_GIFT_CODE_REQ), new(SearchGiftCode))
}
