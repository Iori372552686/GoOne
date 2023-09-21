package misc

import (
	"GoOne/lib/service/svrinstmgr"
	"time"
)

const (
	ClientHeartBeatInterval = 5 * time.Second
	ClientExpiryThreshold   = 4 * ClientHeartBeatInterval //10 * ClientHeartBeatInterval
	MaxTransNumber          = 10000
	MainSvrTickInterval     = 100 * time.Millisecond
)

const (
	ServerType_ConnSvr   = 1  // 连接服
	ServerType_MainSvr   = 2  // 主逻辑服，玩家的大部分单机逻辑都在这里
	ServerType_MysqlSvr  = 3  // 读写mysql
	ServerType_GMConnsvr = 4  // 类似于Connsvr，GM工具连接这个Connsvr
	ServerType_InfoSvr   = 5  // 获取玩家简要信息的服，玩家数据分三个层级：1 RoleInfo（玩家详细数据）2.BriefInfo（玩家简要信息，客户端查看别人数据的时候拉取） 3.IconDesc （展示玩家头像需要的数据）
	ServerType_MailSvr   = 6  // 邮件服
	ServerType_ChatSvr   = 7  // 聊天服
	ServerType_FriendSvr = 8  // 好友服
	ServerType_RankSvr   = 9  // 排行榜
	ServerType_GuildSvr  = 10 // 公会
)

// 各种类型svr的路由规则配置
var ServerRouteRules = map[uint32]uint32{
	// ServerType_ConnSvr: svrinstmgr.SvrRouterRule_Random,  // connsvr 不应该受路由规则限制
	ServerType_MainSvr:   svrinstmgr.SvrRouterRule_UID,
	ServerType_MysqlSvr:  svrinstmgr.SvrRouterRule_Random,
	ServerType_InfoSvr:   svrinstmgr.SvrRouterRule_UID,
	ServerType_MailSvr:   svrinstmgr.SvrRouterRule_UID,
	ServerType_ChatSvr:   svrinstmgr.SvrRouterRule_Random,
	ServerType_FriendSvr: svrinstmgr.SvrRouterRule_UID,
	ServerType_RankSvr:   svrinstmgr.SvrRouterRule_Master,
	ServerType_GuildSvr:  svrinstmgr.SvrRouterRule_UID,
}
