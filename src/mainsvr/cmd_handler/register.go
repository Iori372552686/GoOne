package cmd_handler

import (
	"GoOne/lib/api/logger"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/mainsvr/globals"
)

// 所有的命令字对应的go需要在这里先注册
func RegisterCmd() {
	logger.Infof("Register commands")
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), new(Login))   // 不用NewRoleAdapter，因为会涉及创建角色。
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ), new(Logout)) // 不用NewRoleAdapter，因为如果已经下线了，就不需要LoadRole了。
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ), NewRoleAdapter(new(HeartBeat)))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_CHANGE_NAME_REQ), NewRoleAdapter(new(ChangeName)))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_GM_GET_ROLE_REQ), new(GmGetRole))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_GM_SET_ROLE_REQ), NewRoleAdapter(new(GmSetRole)))
	globals.TransMgr.RegisterCmd(uint32(g1_protocol.CMD_MAIN_GM_ADD_ITEM_REQ), NewRoleAdapter(new(GmAddItem)))

}
