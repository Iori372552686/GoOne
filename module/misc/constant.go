package misc

import (
	"github.com/Iori372552686/GoOne/lib/service/svrinstmgr"
	"time"
)

const (
	ClientHeartBeatInterval = 5 * time.Second
	ClientExpiryThreshold   = 6 * ClientHeartBeatInterval //10 * ClientHeartBeatInterval
	MaxTransNumber          = 10000
	MainSvrTickInterval     = 100 * time.Millisecond
	MaxOrmLruCacheLimitNum  = 100
)

const (
	ServerType_ConnSvr       = 1  // 网关，连接服
	ServerType_MainSvr       = 2  // 主逻辑服，玩家的大部分单机逻辑都在这里
	ServerType_InfoSvr       = 3  // 获取玩家简要信息的服，玩家数据分三个层级：1 RoleInfo（玩家详细数据）2.BriefInfo（玩家简要信息，客户端查看别人数据的时候拉取） 3.IconDesc （展示玩家头像需要的数据）
	ServerType_MysqlSvr      = 4  // 读写mysql，orm
	ServerType_GmSvr         = 5  // 类似于wsConn，提供http restApi接口，GM工具与运营系统连接这个Connsvr
	ServerType_MailSvr       = 6  // 邮件服
	ServerType_ChatSvr       = 7  // 聊天服
	ServerType_FriendSvr     = 8  // 好友服
	ServerType_RankSvr       = 9  // 排行榜
	ServerType_GuildSvr      = 10 // 公会（俱乐部）
	ServerType_RoomCenterSvr = 11 // 房间中心服

	//----- 游戏玩法服务 start-----
	ServerType_TexasGameSvr = 0x50 // porker 德州游戏服
	ServerType_RummyGameSvr = 0x51 // porker 拉米牌游戏服
	//----- 游戏玩法服务 end-----
)

// 各种类型svr的路由规则配置
var ServerRouteRules = map[uint32]uint32{
	// ServerType_ConnSvr: svrinstmgr.SvrRouterRule_Random,  // connsvr 不应该受路由规则限制
	ServerType_MainSvr:       svrinstmgr.SvrRouterRule_Hash_UID,
	ServerType_MysqlSvr:      svrinstmgr.SvrRouterRule_Hash_RouterID,
	ServerType_GmSvr:         svrinstmgr.SvrRouterRule_Random,
	ServerType_InfoSvr:       svrinstmgr.SvrRouterRule_Hash_UID,
	ServerType_MailSvr:       svrinstmgr.SvrRouterRule_Hash_UID,
	ServerType_ChatSvr:       svrinstmgr.SvrRouterRule_Random,
	ServerType_FriendSvr:     svrinstmgr.SvrRouterRule_Hash_UID,
	ServerType_RankSvr:       svrinstmgr.SvrRouterRule_Hash_ZoneID,
	ServerType_RoomCenterSvr: svrinstmgr.SvrRouterRule_Hash_RouterID,
	ServerType_GuildSvr:      svrinstmgr.SvrRouterRule_Hash_RouterID,
	ServerType_TexasGameSvr:  svrinstmgr.SvrRouterRule_Hash_RouterID,
	ServerType_RummyGameSvr:  svrinstmgr.SvrRouterRule_Hash_RouterID,
}
