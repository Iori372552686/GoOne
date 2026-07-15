package service

import (
	"strconv"

	"github.com/Iori372552686/GoOne/common/gamedata/repository/mall_config"
	"github.com/Iori372552686/GoOne/common/gamedata/repository/texas_config"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/Iori372552686/GoOne/src/mainsvr/room"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MainC2SServiceImpl is the IDL-driven ssrpc implementation for mainsvr client commands.
type MainC2SServiceImpl struct{}

func (s *MainC2SServiceImpl) Login(ctx *ssrpc.Context, req *g1_protocol.LoginReq) (*g1_protocol.LoginRsp, error) {
	_ = req

	ctx.Infof("---------------  Login  %d     ---------------", ctx.Uid())

	rsp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	myRole := globals.RoleMgr.GetOrLoadOrCreateRole(ctx.Uid(), ctx)
	if myRole == nil {
		ctx.Errorf("Failed to get role. {req:%v}", req)
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER, "biz_error", "")
	}

	processConnSvrInfo(ctx, myRole)
	now := myRole.Now()
	myRole.OnLogin(now)
	// 时间是0 就没有上次登录的时间
	myRole.PbRole.LoginInfo.LastLoginTime = myRole.PbRole.LoginInfo.NowLoginTime
	myRole.PbRole.LoginInfo.NowLoginTime = now
	myRole.OnClientHeartbeat(now)
	myRole.AfterLogin(now)

	// rsp
	rsp.TimeNowMs = myRole.NowMs()
	rsp.RoleInfo = new(g1_protocol.RoleInfo)
	proto.Merge(rsp.RoleInfo, myRole.PbRole)
	ctx.Infof("role login {uid:%d, role_size:%d}", ctx.Uid(), proto.Size(rsp.RoleInfo))
	_ = myRole.FlushPending(ctx, false)

	return rsp, nil
}

func (s *MainC2SServiceImpl) Logout(ctx *ssrpc.Context, req *g1_protocol.LogoutReq) (*g1_protocol.LogoutRsp, error) {
	ret := g1_protocol.ErrorCode_ERR_OK

	myRole := globals.RoleMgr.GetRole(ctx.Uid())
	if myRole == nil {
		ctx.Debugf("Already logged out. {req=%#v}", req)
		return &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER}}, nil
	}

	myRole.PbRole.LoginInfo.LastLogoutTime = myRole.Now()
	myRole.SaveToDB(ctx)
	globals.RoleMgr.DeleteRole(ctx.Uid())

	// If server-triggered logout, legacy behavior: no rsp.
	if req.GetByServer() {
		ctx.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", ctx.Uid(), req.GetByServer(), req.GetReason())
		return nil, nil
	}

	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{Code: ret}}
	ctx.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", ctx.Uid(), req.GetByServer(), req.GetReason())
	return rsp, nil
}

func (s *MainC2SServiceImpl) HeartBeat(ctx *ssrpc.Context, req *g1_protocol.HeartBeatReq) (*g1_protocol.HeartBeatRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	ret := g1_protocol.ErrorCode_ERR_OK
	now := myRole.Now()
	myRole.OnClientHeartbeat(now)
	_ = myRole.FlushPending(ctx, false)

	rsp := &g1_protocol.HeartBeatRsp{
		ClientNowMsInReq: req.GetClientNowMs(),
		ServerNowMs:      myRole.NowMs(),
		Ret:              &g1_protocol.Ret{Code: ret},
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) ChangeName(ctx *ssrpc.Context, req *g1_protocol.ChangeNameReq) (*g1_protocol.ChangeNameRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.ChangeNameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	// 检查是否满足条件
	free := myRole.PbRole.BasicInfo.GetFreeCnt()
	_, hasCoin := myRole.ItemCheckReduce(int32(g1_protocol.EItemID_GOLD), 100)
	if hasCoin != 0 && free <= 0 {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH, "biz_error", "")
	}

	// 检查敏感字
	hasSensitiveWord, _ := sensitive_words.ChangeSensitiveWords(req.GetName())
	if hasSensitiveWord {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INVALID_NAME, "biz_error", "")
	}

	// 同步数据
	myRole.PbRole.BasicInfo.Name = req.GetName()
	myRole.TouchBasicInfo("change_name")
	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}

func (s *MainC2SServiceImpl) ChangeIcon(ctx *ssrpc.Context, req *g1_protocol.ChangeIconReq) (*g1_protocol.ChangeIconRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.ChangeIconRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	if req.GetIconId() > 0 {
		code := myRole.IconChange(req.GetIconId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "icon_change", "")
		}
	}
	if req.GetFrameId() > 0 {
		code := myRole.FrameChange(req.GetFrameId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "frame_change", "")
		}
	}
	if req.GetImageId() > 0 {
		code := myRole.ImageChange(req.GetImageId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "image_change", "")
		}
	}

	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}

func (s *MainC2SServiceImpl) GmGetRole(ctx *ssrpc.Context, req *g1_protocol.GMGetRoleReq) (*g1_protocol.GMGetRoleRsp, error) {
	_ = req
	ret := g1_protocol.ErrorCode_ERR_OK
	rsp := &g1_protocol.GMGetRoleRsp{}

	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		ctx.Infof("Gm try to get not existing role.")
		ret = g1_protocol.ErrorCode_ERR_DB
		rsp.Ret = &g1_protocol.Ret{Code: ret}
		return rsp, nil
	}

	rsp.RoleInfo = new(g1_protocol.RoleInfo)
	proto.Merge(rsp.RoleInfo, myRole.PbRole)
	logger.Infof("GM get role {uid: %d}", ctx.Uid())

	rsp.Ret = &g1_protocol.Ret{Code: ret}
	return rsp, nil
}

func (s *MainC2SServiceImpl) GmSetRole(ctx *ssrpc.Context, req *g1_protocol.GMSetRoleReq) (*g1_protocol.GMSetRoleRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	if req.GetRoleInfo() == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_MARSHAL, "missing role_info")
	}

	// 先保存 connbus
	connBus := uint32(0)
	if myRole.PbRole != nil && myRole.PbRole.ConnSvrInfo != nil {
		connBus = myRole.PbRole.ConnSvrInfo.BusId
	}

	myRole.PbRole = req.GetRoleInfo()
	if myRole.PbRole.ConnSvrInfo == nil {
		myRole.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	myRole.PbRole.ConnSvrInfo.BusId = connBus

	logger.Infof("GM set role {uid:%d, role_size:%d}", ctx.Uid(), proto.Size(myRole.PbRole))
	myRole.MarkFullSync(g1_protocol.ERoleSectionFlag_ALL)
	myRole.MarkPersistDirty("gm_set_role")
	_ = myRole.FlushPending(ctx, true)

	return &g1_protocol.GMSetRoleRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}, nil
}

func (s *MainC2SServiceImpl) GmAddItem(ctx *ssrpc.Context, req *g1_protocol.GMAddItemReq) (*g1_protocol.GMAddItemRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	ret := myRole.ItemAdd(req.GetId(), req.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_GM, Scene: 0})
	_ = myRole.FlushPending(ctx, false)
	return &g1_protocol.GMAddItemRsp{Ret: &g1_protocol.Ret{Code: ret}}, nil
}

func (s *MainC2SServiceImpl) MallBuyPackage(ctx *ssrpc.Context, req *g1_protocol.MallBuyPackageReq) (*g1_protocol.MallBuyPackageRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.MallBuyPackageRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	code := myRole.MallCheckBuyCondition(req.GetConfId())
	if code != g1_protocol.ErrorCode_ERR_OK {
		return rsp, gerr.New(code, "mall_check", "")
	}

	conf := mall_config.GetById(req.GetConfId())
	if conf == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_CONF, "conf_not_found", "")
	}

	_, code = myRole.ItemCheckReduce(conf.CostItemID, int64(conf.CostItemCnt))
	if code != g1_protocol.ErrorCode_ERR_OK {
		return rsp, gerr.New(code, "item_check_reduce", "")
	}

	// 如果是充值购买的礼包就走充值（暂未实现，保持旧逻辑）
	if int32(g1_protocol.EItemID_ACECOIN) == conf.CostItemID {
		// ret = RechargeAdd(conf.Rmb, myRole)
	} else {
		code = myRole.ItemExchange(conf.CostItemID, int64(conf.CostItemCnt), conf.PackageID,
			1, &role.Reason{Reason: g1_protocol.Reason_REASON_MALL_PACKAGE, Scene: req.GetConfId()})
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "item_exchange", "")
		}
	}

	myRole.MallAddBuyCount(req.GetConfId())
	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}

func (s *MainC2SServiceImpl) CreateRoom(ctx *ssrpc.Context, req *g1_protocol.CreateRoomReq) (*g1_protocol.CreateRoomRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainCreateRoom(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) JoinRoom(ctx *ssrpc.Context, req *g1_protocol.JoinRoomReq) (*g1_protocol.JoinRoomRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainJoinRoom(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) QuickStart(ctx *ssrpc.Context, req *g1_protocol.QuickStartReq) (*g1_protocol.QuickStartRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainQuickStart(ctx, req, myRole), nil
}

func (s *MainC2SServiceImpl) GetRoomList(ctx *ssrpc.Context, req *g1_protocol.RoomListReq) (*g1_protocol.RoomListRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	return room.OnMainGetRoomList(ctx, req, myRole), nil
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

func (s *MainC2SServiceImpl) MainBuyInDetail(ctx *ssrpc.Context, req *g1_protocol.MainBuyInDetailReq) (*g1_protocol.MainBuyInDetailRsp, error) {
	_ = ctx
	rsp := &g1_protocol.MainBuyInDetailRsp{Ret: &g1_protocol.Ret{}}
	cfg := texas_config.GetByRoomStageCoinType(req.GetRoomStage(), int32(req.GetCoinType()))
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

func (s *MainC2SServiceImpl) GetLookers(ctx *ssrpc.Context, req *g1_protocol.GetLookersReq) (*g1_protocol.GetLookersRsp, error) {
	rsp := &g1_protocol.GetLookersRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_LOOKERS_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) SitDown(ctx *ssrpc.Context, req *g1_protocol.SitDownReq) (*g1_protocol.SitDownRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
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

func (s *MainC2SServiceImpl) StandUp(ctx *ssrpc.Context, req *g1_protocol.StandUpReq) (*g1_protocol.StandUpRsp, error) {
	rsp := &g1_protocol.StandUpRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_STAND_UP_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) LeaveGame(ctx *ssrpc.Context, req *g1_protocol.LeaveGameReq) (*g1_protocol.LeaveGameRsp, error) {
	myRole := globals.RoleMgr.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return &g1_protocol.LeaveGameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_ARGV}}, nil
	}
	return room.OnMainExitRoom(ctx, req, myRole), nil
}

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
func (s *MainC2SServiceImpl) GetRoomInfo(ctx *ssrpc.Context, req *g1_protocol.GetRoomInfoReq) (*g1_protocol.GetRoomInfoRsp, error) {
	rsp := &g1_protocol.GetRoomInfoRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_ROOM_INFO_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
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
func (s *MainC2SServiceImpl) GetGameInfo(ctx *ssrpc.Context, req *g1_protocol.GetGameInfoReq) (*g1_protocol.GetGameInfoRsp, error) {
	rsp := &g1_protocol.GetGameInfoRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_GET_GAME_INFO_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
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
func (s *MainC2SServiceImpl) Preoperation(ctx *ssrpc.Context, req *g1_protocol.PreOperationReq) (*g1_protocol.PreOperationRsp, error) {
	rsp := &g1_protocol.PreOperationRsp{Ret: &g1_protocol.Ret{}}
	err := ctx.CallMsgByRouter(misc.ServerType_TexasGameSvr, req.GetRoomId(), g1_protocol.CMD_TEXAS_INNER_PREOPERATION_REQ, req, rsp)
	if err != nil {
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "forward_failed", err)
	}
	return rsp, nil
}

func processConnSvrInfo(c interface {
	Uid() uint64
	OriSrcBusId() uint32
	Ip() uint32
	Flag() uint32
}, myRole *role.Role) {
	connSvrInfo := myRole.PbRole.ConnSvrInfo

	ipStr := bus.IpIntToString(c.Ip())
	portStr := strconv.Itoa(int(c.Flag())) // 端口是存在flag字段里面的
	remoteAddr := ipStr + ":" + portStr

	connSvrInfo.BusId = c.OriSrcBusId()
	connSvrInfo.ClientPos = remoteAddr
}
