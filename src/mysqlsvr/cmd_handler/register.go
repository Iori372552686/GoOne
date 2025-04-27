package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// 所有的命令字对应的go需要在这里先注册
func RegCmd() {
	logger.Infof("register transaction commands")
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_UPDATE_ROLE_INFO_REQ, UpdateRoleInfo)
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_SEARCH_ROLE_REQ, SearchRole)
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_UPDATE_REQ, UpdateRequest)                     // 更新数据库
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_QUERY_ROOM_INFO_REQ, QueryRoomInfoRequest)     // 查询房间信息
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_QUERY_PLAYER_INFO_REQ, QueryPlayerInfoRequest) // 查询玩家信息
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MYSQL_INNER_QUERY_GAME_INFO_REQ, QueryGameInfoRequest)     // 查询游戏信息
}
