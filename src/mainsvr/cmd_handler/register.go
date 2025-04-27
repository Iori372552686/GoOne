package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// 所有的命令字对应的go需要在这里先注册
func RegisterCmd() {
	logger.Infof("Register commands")
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_LOGIN_REQ, Login)   // 不用NewRoleAdapter，因为会涉及创建角色。
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_LOGOUT_REQ, Logout) // 不用NewRoleAdapter，因为如果已经下线了，就不需要LoadRole了。
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_HEARTBEAT_REQ, NewRoleAdapter(HeartBeat))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_CHANGE_NAME_REQ, NewRoleAdapter(ChangeName))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_CHANGE_ICON_REQ, NewRoleAdapter(ChangeIcon))

	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GM_GET_ROLE_REQ, GmGetRole)
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GM_SET_ROLE_REQ, NewRoleAdapter(GmSetRole))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GM_ADD_ITEM_REQ, NewRoleAdapter(GmAddItem))
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_MALL_BUY_PACKAGE_REQ, NewRoleAdapter(MallBuyPackage))
	//globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_MALL_RECHARGE_REQ, NewRoleAdapter(MallRecharge))

	//------- 德州游戏房间操作  start--------
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_CREATE_ROOM_REQ, NewRoleAdapter(CreateRoom))                  //创建德州房间
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_JOIN_ROOM_REQ, NewRoleAdapter(JoinRoom))                      //加入房间
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_QUICK_START_REQ, NewRoleAdapter(QuickStart))                  //快速开始
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ, NewRoleAdapter(GetRoomList))                   //get房间列表
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_DO_BET_REQ, NewRoleAdapter(DoBet))                            //下注
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_FOLD_REQ, NewRoleAdapter(Fold))                               //Fold
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_BUY_IN_DETAIL_REQ, NewRoleAdapter(MainBuyInDetail))           //买入明细
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_LOOKERS_REQ, NewRoleAdapter(GetLookers))                  //get lookers
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_SIT_DOWN_REQ, NewRoleAdapter(SitDown))                        //SitDown
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_STAND_UP_REQ, NewRoleAdapter(StandUp))                        //StandUp
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_LEAVE_GAME_REQ, NewRoleAdapter(LeaveGame))                    //LeaveGame
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_MILITARY_SUCCESS_REQ, NewRoleAdapter(MilitarySuccess))        //战绩请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_GAME_LOG_REQ, NewRoleAdapter(GetGameLog))                 //获取游戏记录
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_TIME_LEFT_REQ, NewRoleAdapter(GetTimeLeft))               //获取剩余时间
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_VOICE_CALL_REQ, NewRoleAdapter(VoiceCall))                    //语音通话
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_BUY_THINK_TIME_REQ, NewRoleAdapter(BuyThinkTime))             //购买思考时间
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_AUTO_BUYIN_REQ, NewRoleAdapter(AutoBuyin))                    //自动买入设置
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_INTERACTION_REQ, NewRoleAdapter(Interaction))                 //互动请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_EMOTICON_REQ, NewRoleAdapter(Emoticon))                       //表情发送请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_BUY_IN_REQ, NewRoleAdapter(BuyIn))                            //买入请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_MILITARY_DIAGRAM_REQ, NewRoleAdapter(GetMilitaryDiagram)) //请求战绩折线图
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_SHOW_CARD_REQ, NewRoleAdapter(ShowCard))                      //翻牌展示请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_ROLE_INFO_REQ, NewRoleAdapter(GetPlayerInfo))             //请求玩家详细信息
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_MARK_PLAYER_REQ, NewRoleAdapter(MarkPlayer))                  //标记玩家请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_INSURANCE_BUY_REQ, NewRoleAdapter(InsuranceBuy))              //购买保险请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_ROOM_SET_REQ, NewRoleAdapter(RoomSet))                        //房间设置修改
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_SNG_GET_BLIND_LEVEL_REQ, NewRoleAdapter(SngGetBlindLevel))    //请求盲注等级
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_ROOM_INFO_REQ, NewRoleAdapter(GetRoomInfo))               //请求房间详情
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_INSURANCE_THINK_TIME_REQ, NewRoleAdapter(InsuranceThinkTime)) //保险思考时间查询
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_INSURANCE_OP_REQ, NewRoleAdapter(InsuranceOp))                //保险操作请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_GET_GAME_INFO_REQ, NewRoleAdapter(GetGameInfo))               //弱网时获取牌局信息
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_ADD_TO_FAVORITE_REQ, NewRoleAdapter(AddToFavorite))           //添加收藏请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_CHANGE_SKIN_REQ, NewRoleAdapter(ChangeSkin))                  //更换皮肤请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_RABBIT_HUNTING_REQ, NewRoleAdapter(RabbitHunting))            //特殊活动请求
	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_EARLY_SETTLE_REQ, NewRoleAdapter(EarlySettle))                //提前结算请求

	globals.TransMgr.RegisterCmd(g1_protocol.CMD_MAIN_GAME_PREOPERATION_REQ, NewRoleAdapter(Preoperation)) //预操作指令提交
	//------- 德州游戏房间操作  end--------
}
