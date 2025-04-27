package cmd_handler

import (
	"fmt"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"

	"github.com/Iori372552686/GoOne/common/gamedata/repository/texas_config"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/room"
)

func CreateRoom(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.CreateRoomReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	rsp := room.OnMainCreatRoom(c, req, myRole)
	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func JoinRoom(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.JoinRoomReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	rsp := room.OnMainJoinRoom(c, req, myRole)
	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func QuickStart(c cmd_handler.IContext, data []byte, role *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.QuickStartReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	rsp := room.OnMainQuickStart(c, req, role)
	c.SendMsgBack(rsp)
	return rsp.Ret.Code

}

func GetRoomList(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.RoomListReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	rsp := room.OnMainGetRoomList(c, req, myRole)
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func DoBet(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.DoBetReq{}
	rsp := &g1_protocol.DoBetRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_DO_BET_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func Fold(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.FoldReq{}
	rsp := &g1_protocol.FoldRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_FOLD_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func MainBuyInDetail(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.MainBuyInDetailReq{}
	rsp := &g1_protocol.MainBuyInDetailRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	cfg := texas_config.GetByRoomStageCoinType(req.RoomStage, int32(req.CoinType))
	if cfg == nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_CONF
		rsp.Ret.Msg = fmt.Sprintf("??????: stage:%d, coinType:%d", req.RoomStage, req.CoinType)
	} else {
		rsp.SmallBlind = cfg.SmallBlind
		rsp.BigBlind = cfg.BigBlind
		rsp.MaxBuyin = cfg.MaxBuyIn
		rsp.MinBuyin = cfg.MinBuyIn
	}

	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetLookers(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetLookersReq{}
	rsp := &g1_protocol.GetLookersRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_GET_LOOKERS_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func SitDown(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.SitDownReq{}
	rsp := &g1_protocol.SitDownRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	req.RoleIcon = myRole.GetIconDesc()
	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_SIT_DOWN_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func StandUp(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.StandUpReq{}
	rsp := &g1_protocol.StandUpRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_STAND_UP_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return rsp.Ret.Code
}

func AutoBuyin(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.AutoBuyinReq{}
	rsp := &g1_protocol.AutoBuyinRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func Interaction(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.InteractionReq{}
	rsp := &g1_protocol.InteractionRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func Emoticon(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.EmoticonReq{}
	rsp := &g1_protocol.EmoticonRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func BuyIn(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.BuyInReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	//rsp := room.OnBuyin(c, req, myRole)
	//c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetMilitaryDiagram(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetMilitaryDiagramReq{}
	rsp := &g1_protocol.GetMilitaryDiagramRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func ShowCard(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.ShowCardReq{}
	rsp := &g1_protocol.ShowCardRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetPlayerInfo(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetPlayerInfoReq{}
	rsp := &g1_protocol.GetPlayerInfoRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func MarkPlayer(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.MarkPlayerReq{}
	rsp := &g1_protocol.MarkPlayerRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func InsuranceBuy(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.InsuranceBuyReq{}
	rsp := &g1_protocol.InsuranceBuyRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func RoomSet(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.RoomSetReq{}
	rsp := &g1_protocol.RoomSetRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func SngGetBlindLevel(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.SngGetBlindLevelReq{}
	rsp := &g1_protocol.SngGetBlindLevelRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetRoomInfo(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetRoomInfoReq{}
	rsp := &g1_protocol.GetRoomInfoRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_GET_ROOM_INFO_REQ, rsp, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	return g1_protocol.ErrorCode_ERR_OK
}

func InsuranceThinkTime(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.InsuranceThinkTimeReq{}
	rsp := &g1_protocol.InsuranceThinkTimeRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func InsuranceOp(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.InsuranceOpReq{}
	rsp := &g1_protocol.InsuranceOpRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetGameInfo(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetGameInfoReq{}
	rsp := &g1_protocol.GetGameInfoRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_GET_GAME_INFO_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func AddToFavorite(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.AddToFavoriteReq{}
	rsp := &g1_protocol.AddToFavoriteRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func ChangeSkin(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.ChangeSkinReq{}
	rsp := &g1_protocol.ChangeSkinRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func Preoperation(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.PreOperationReq{}
	rsp := &g1_protocol.PreOperationRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.RoomId, g1_protocol.CMD_TEXAS_INNER_PREOPERATION_REQ, req, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func RabbitHunting(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.RabbitHuntingReq{}
	rsp := &g1_protocol.RabbitHuntingRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func EarlySettle(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.EarlySettleReq{}
	rsp := &g1_protocol.EarlySettleRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func LeaveGame(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.LeaveGameReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	c.SendMsgBack(room.OnMainExitRoom(c, req, myRole))
	return g1_protocol.ErrorCode_ERR_OK
}

func MilitarySuccess(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.MilitarySuccessReq{}
	rsp := &g1_protocol.MilitarySuccessRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetGameLog(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetGameLogReq{}
	rsp := &g1_protocol.GetGameLogRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func GetTimeLeft(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GetTimeLeftReq{}
	rsp := &g1_protocol.GetTimeLeftRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func VoiceCall(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.VoiceCallReq{}
	rsp := &g1_protocol.VoiceCallRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func BuyThinkTime(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.BuyThinkTimeReq{}
	rsp := &g1_protocol.BuyThinkTimeRsp{Ret: &g1_protocol.Ret{}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}
