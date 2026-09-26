package service

import (
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/gamedata/repository/texas"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/room"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ===== 房间入口与对局转发 =====
// 房间目录交互（快速开始/列表/回滚）经 room/ 用例包转发到 roomcentersvr；
// 对局操作直接按 RoomId 路由到 TexasGameSvr（当前 checkout 不含游戏服实现）。

func (s *MainC2SServiceImpl) CreateRoom(ctx *ssrpc.Context, req *g1_protocol.CreateRoomReq) (*g1_protocol.CreateRoomRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainCreateRoom(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) JoinRoom(ctx *ssrpc.Context, req *g1_protocol.JoinRoomReq) (*g1_protocol.JoinRoomRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainJoinRoom(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) QuickStart(ctx *ssrpc.Context, req *g1_protocol.QuickStartReq) (*g1_protocol.QuickStartRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainQuickStart(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) GetRoomList(ctx *ssrpc.Context, req *g1_protocol.RoomListReq) (*g1_protocol.RoomListRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainGetRoomList(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) LeaveGame(ctx *ssrpc.Context, req *g1_protocol.LeaveGameReq) (*g1_protocol.LeaveGameRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_ARGV}}, nil
	}
	return room.OnMainExitRoom(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) SitDown(ctx *ssrpc.Context, req *g1_protocol.SitDownReq) (*g1_protocol.SitDownRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return &g1_protocol.SitDownRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_ARGV}}, nil
	}
	req.RoleIcon = myRole.GetIconDesc()
	rsp := &g1_protocol.SitDownRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_SIT_DOWN_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

// MainBuyInDetail 查买入档位配置（本地 gamedata，不经游戏服）。
func (s *MainC2SServiceImpl) MainBuyInDetail(ctx *ssrpc.Context, req *g1_protocol.MainBuyInDetailReq) (*g1_protocol.MainBuyInDetailRsp, error) {
	_ = ctx
	rsp := &g1_protocol.MainBuyInDetailRsp{Ret: &g1_protocol.Ret{}}
	cfg := texas.GetTexasByRoomStageCoinType(req.GetRoomStage(), int32(req.GetCoinType()))
	if cfg == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_CONF, "conf_not_found",
			"missing texas config: stage=%d coinType=%d", req.GetRoomStage(), req.GetCoinType())
	}
	rsp.SmallBlind = cfg.SmallBlind
	rsp.BigBlind = cfg.BigBlind
	rsp.MaxBuyin = cfg.MaxBuyIn
	rsp.MinBuyin = cfg.MinBuyIn
	return rsp, nil
}

func (s *MainC2SServiceImpl) DoBet(ctx *ssrpc.Context, req *g1_protocol.DoBetReq) (*g1_protocol.DoBetRsp, error) {
	rsp := &g1_protocol.DoBetRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_DO_BET_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) Fold(ctx *ssrpc.Context, req *g1_protocol.FoldReq) (*g1_protocol.FoldRsp, error) {
	rsp := &g1_protocol.FoldRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_FOLD_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) GetLookers(ctx *ssrpc.Context, req *g1_protocol.GetLookersReq) (*g1_protocol.GetLookersRsp, error) {
	rsp := &g1_protocol.GetLookersRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_LOOKERS_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) StandUp(ctx *ssrpc.Context, req *g1_protocol.StandUpReq) (*g1_protocol.StandUpRsp, error) {
	rsp := &g1_protocol.StandUpRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_STAND_UP_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) GetRoomInfo(ctx *ssrpc.Context, req *g1_protocol.GetRoomInfoReq) (*g1_protocol.GetRoomInfoRsp, error) {
	rsp := &g1_protocol.GetRoomInfoRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_ROOM_INFO_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) GetGameInfo(ctx *ssrpc.Context, req *g1_protocol.GetGameInfoReq) (*g1_protocol.GetGameInfoRsp, error) {
	rsp := &g1_protocol.GetGameInfoRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_GAME_INFO_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) Preoperation(ctx *ssrpc.Context, req *g1_protocol.PreOperationReq) (*g1_protocol.PreOperationRsp, error) {
	rsp := &g1_protocol.PreOperationRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_PREOPERATION_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

// ===== 对局桩（协议占位：返回 OK，待玩法接入后补实现） =====

func (s *MainC2SServiceImpl) BuyIn(ctx *ssrpc.Context, req *g1_protocol.BuyInReq) (*emptypb.Empty, error) {
	_ = ctx
	_ = req
	// Legacy handler returns OK without sending response; keep one-way stub.
	return nil, nil
}

func (s *MainC2SServiceImpl) MilitarySuccess(ctx *ssrpc.Context, req *g1_protocol.MilitarySuccessReq) (*g1_protocol.MilitarySuccessRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.MilitarySuccessRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) GetGameLog(ctx *ssrpc.Context, req *g1_protocol.GetGameLogReq) (*g1_protocol.GetGameLogRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.GetGameLogRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) GetTimeLeft(ctx *ssrpc.Context, req *g1_protocol.GetTimeLeftReq) (*g1_protocol.GetTimeLeftRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.GetTimeLeftRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) VoiceCall(ctx *ssrpc.Context, req *g1_protocol.VoiceCallReq) (*g1_protocol.VoiceCallRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.VoiceCallRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) BuyThinkTime(ctx *ssrpc.Context, req *g1_protocol.BuyThinkTimeReq) (*g1_protocol.BuyThinkTimeRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.BuyThinkTimeRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) AutoBuyin(ctx *ssrpc.Context, req *g1_protocol.AutoBuyinReq) (*g1_protocol.AutoBuyinRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.AutoBuyinRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) Interaction(ctx *ssrpc.Context, req *g1_protocol.InteractionReq) (*g1_protocol.InteractionRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.InteractionRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) Emoticon(ctx *ssrpc.Context, req *g1_protocol.EmoticonReq) (*g1_protocol.EmoticonRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.EmoticonRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) GetMilitaryDiagram(ctx *ssrpc.Context, req *g1_protocol.GetMilitaryDiagramReq) (*g1_protocol.GetMilitaryDiagramRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.GetMilitaryDiagramRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) ShowCard(ctx *ssrpc.Context, req *g1_protocol.ShowCardReq) (*g1_protocol.ShowCardRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.ShowCardRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) GetPlayerInfo(ctx *ssrpc.Context, req *g1_protocol.GetPlayerInfoReq) (*g1_protocol.GetPlayerInfoRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.GetPlayerInfoRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) MarkPlayer(ctx *ssrpc.Context, req *g1_protocol.MarkPlayerReq) (*g1_protocol.MarkPlayerRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.MarkPlayerRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) InsuranceBuy(ctx *ssrpc.Context, req *g1_protocol.InsuranceBuyReq) (*g1_protocol.InsuranceBuyRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.InsuranceBuyRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) RoomSet(ctx *ssrpc.Context, req *g1_protocol.RoomSetReq) (*g1_protocol.RoomSetRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.RoomSetRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) SngGetBlindLevel(ctx *ssrpc.Context, req *g1_protocol.SngGetBlindLevelReq) (*g1_protocol.SngGetBlindLevelRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.SngGetBlindLevelRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) InsuranceThinkTime(ctx *ssrpc.Context, req *g1_protocol.InsuranceThinkTimeReq) (*g1_protocol.InsuranceThinkTimeRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.InsuranceThinkTimeRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) InsuranceOp(ctx *ssrpc.Context, req *g1_protocol.InsuranceOpReq) (*g1_protocol.InsuranceOpRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.InsuranceOpRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) AddToFavorite(ctx *ssrpc.Context, req *g1_protocol.AddToFavoriteReq) (*g1_protocol.AddToFavoriteRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.AddToFavoriteRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) ChangeSkin(ctx *ssrpc.Context, req *g1_protocol.ChangeSkinReq) (*g1_protocol.ChangeSkinRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.ChangeSkinRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) RabbitHunting(ctx *ssrpc.Context, req *g1_protocol.RabbitHuntingReq) (*g1_protocol.RabbitHuntingRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.RabbitHuntingRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}

func (s *MainC2SServiceImpl) EarlySettle(ctx *ssrpc.Context, req *g1_protocol.EarlySettleReq) (*g1_protocol.EarlySettleRsp, error) {
	_ = ctx
	_ = req
	return &g1_protocol.EarlySettleRsp{Ret: &g1_protocol.Ret{Code: 0}}, nil
}
