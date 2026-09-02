package service

import (
	"context"
	"errors"
	"time"

	"github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/repository"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

const asyncWriteTimeout = 15 * time.Second

type MysqlServiceImpl struct {
	mysqlsvrv1.MysqlServiceSS
	repo repository.Store
}

func NewMysqlServiceImpl(repo repository.Store) *MysqlServiceImpl {
	return &MysqlServiceImpl{repo: repo}
}

func (s *MysqlServiceImpl) UpdateRoleInfo(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerUpdateRoleInfoReq) (*g1_protocol.MysqlInnerUpdateRoleInfoRsp, error) {
	rsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if ctx == nil || s.repo == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	if err := s.repo.UpdateRole(requestContext(ctx), ctx.Uid(), req.GetName()); err != nil {
		logger.Errorf("failed to update role info | %v", err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "update_role", err)
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) SearchRole(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerSearchRoleReq) (*g1_protocol.MysqlInnerSearchRoleRsp, error) {
	rsp := &g1_protocol.MysqlInnerSearchRoleRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if s.repo == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	uid, err := s.repo.SearchRole(requestContext(ctx), req.GetSearchString())
	if err != nil {
		logger.Errorf("failed to select role info: %v", err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "search_role", err)
	}
	rsp.Uid = uid
	return rsp, nil
}

func (s *MysqlServiceImpl) Update(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerUpdateReq) (*emptypb.Empty, error) {
	if s.repo == nil {
		return &emptypb.Empty{}, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	var task func(context.Context) error
	switch req.GetDataType() {
	case g1_protocol.DataType_DATA_TYPE_TEXAS_ROOM_INFO:
		item := new(g1_protocol.MysqlTexasRoomInfo)
		if err := proto.Unmarshal(req.GetData(), item); err != nil {
			return &emptypb.Empty{}, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "decode_room_info", err)
		}
		task = func(writeCtx context.Context) error { return s.repo.SaveRoom(writeCtx, item) }
	case g1_protocol.DataType_DATA_TYPE_TEXAS_GAME_RECORD:
		item := new(g1_protocol.MysqlTexasGameInfo)
		if err := proto.Unmarshal(req.GetData(), item); err != nil {
			return &emptypb.Empty{}, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "decode_game_info", err)
		}
		task = func(writeCtx context.Context) error { return s.repo.SaveGame(writeCtx, item) }
	case g1_protocol.DataType_DATA_TYPE_PLAYER_INFO:
		item := new(g1_protocol.MysqlTexasPlayerInfo)
		if err := proto.Unmarshal(req.GetData(), item); err != nil {
			return &emptypb.Empty{}, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "decode_player_info", err)
		}
		task = func(writeCtx context.Context) error { return s.repo.InsertPlayer(writeCtx, item) }
	default:
		return &emptypb.Empty{}, nil
	}

	base := context.WithoutCancel(requestContext(ctx))
	if err := manager.Push(req.GetId(), func() {
		writeCtx, cancel := context.WithTimeout(base, asyncWriteTimeout)
		defer cancel()
		if err := task(writeCtx); err != nil {
			if errors.Is(err, repository.ErrStaleUpdate) {
				logger.Warningf("mysqlsvr rejected stale async update | %v", err)
				return
			}
			logger.Errorf("mysqlsvr async update failed | %v", err)
		}
	}); err != nil {
		return &emptypb.Empty{}, gerr.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "enqueue_db_write", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *MysqlServiceImpl) QueryRoomInfo(ctx *ssrpc.Context, req *g1_protocol.QueryRoomInfoReq) (*g1_protocol.QueryRoomInfoRsp, error) {
	if s.repo == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_INTERNAL, "database repository unavailable")
	}
	items, err := s.repo.QueryRoom(requestContext(ctx), req)
	if err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query room info failed")
	}
	return &g1_protocol.QueryRoomInfoRsp{List: items}, nil
}

func (s *MysqlServiceImpl) QueryPlayerInfo(ctx *ssrpc.Context, req *g1_protocol.QueryPlayerInfoReq) (*g1_protocol.QueryPlayerInfoRsp, error) {
	if s.repo == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_INTERNAL, "database repository unavailable")
	}
	items, err := s.repo.QueryPlayer(requestContext(ctx), req)
	if err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query player info failed")
	}
	return &g1_protocol.QueryPlayerInfoRsp{List: items}, nil
}

func (s *MysqlServiceImpl) QueryGameInfo(ctx *ssrpc.Context, req *g1_protocol.QueryGameInfoReq) (*g1_protocol.QueryGameInfoRsp, error) {
	if s.repo == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_INTERNAL, "database repository unavailable")
	}
	item, err := s.repo.GetGame(requestContext(ctx), req.GetGameId())
	if err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query game info failed")
	}
	rsp := new(g1_protocol.QueryGameInfoRsp)
	if item == nil {
		return rsp, nil
	}
	detail := new(g1_protocol.TexasGameRecordDetail)
	if err := proto.Unmarshal(item.GameDetail, detail); err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "decode game detail failed")
	}
	rsp.Data = &g1_protocol.TexasGameRecord{
		TableId: item.TableId, GameType: item.GameType, RoomStage: item.RoomStage,
		Blind: item.Blind, BeginTime: item.BeginTime, EndTime: item.EndTime,
		TotalPot: item.TotalPot, TotalService: item.TotalService, Detail: detail, Round: item.Round,
	}
	return rsp, nil
}

func requestContext(ctx *ssrpc.Context) context.Context {
	if ctx == nil || ctx.Context == nil {
		return context.Background()
	}
	return ctx.Context
}
