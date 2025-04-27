package room

import (
	"github.com/Iori372552686/GoOne/common/gfunc"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

func OnMainCreatRoom(c cmd_handler.IContext, req *g1_protocol.CreateRoomReq, myRole *role.Role) *g1_protocol.CreateRoomRsp {
	rsp := &g1_protocol.CreateRoomRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	genId, err := globals.IDGen.GenID()
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_GEN_ID
		rsp.Ret.Msg = err.Error()
		return rsp
	}

	roomId := gfunc.GenerateRoomId(genId)
	rpcReq := &g1_protocol.InnerCreateRoomReq{
		Base: &g1_protocol.RoomBaseInfo{
			RoomId:     roomId,
			OwerId:     myRole.Uid(),
			Name:       req.Name,
			GameId:     req.GameId,
			IsPrivate:  req.IsPrivate,
			Blind:      req.Blind,
			Ante:       req.Ante,
			MaxPlayer:  req.ChairNum,
			IsAuth:     req.IsAuth,
			IsRebuy:    req.IsRebuy,
			IsAddon:    req.IsAddon,
			IsInsure:   req.IsInsure,
			GameTime:   req.GameTime,
			StartBb:    req.StartBb,
			StartTime:  req.StartTime,
			Id:         genId,
			ClubId:     req.ClubId,
			Straddle:   req.Straddle,
			IpLimit:    req.IpLimit,
			GpsLimit:   req.GpsLimit,
			Allianceid: req.AllianceId,
			CreateTime: datetime.NowInt64(),
			OwnerInfo:  myRole.GetIconDesc(),
			CoinType:   req.CoinType,
		}}

	err = c.CallMsgByRouter(misc.ServerType_TexasGameSvr, roomId, g1_protocol.CMD_TEXAS_INNER_CREATEROOM_REQ, rpcReq, rsp)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
		rsp.Ret.Msg = err.Error()
	}

	return rsp
}
