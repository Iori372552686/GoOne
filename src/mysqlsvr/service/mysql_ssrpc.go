package service

import (
	"strconv"

	"github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// MysqlServiceImpl is the IDL-driven ssrpc implementation for mysqlsvr internal RPCs.
type MysqlServiceImpl struct{}

var _ mysqlsvrv1.MysqlServiceSS = (*MysqlServiceImpl)(nil)

func (s *MysqlServiceImpl) UpdateRoleInfo(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerUpdateRoleInfoReq) (*g1_protocol.MysqlInnerUpdateRoleInfoRsp, error) {
	rsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if ctx == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}

	engine := globals.OrmMgr.GetOrmEngine()
	if engine == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}

	if mysqlRoleExists(ctx) {
		ctx.Infof("role exist")
		if _, err := engine.Exec("UPDATE role_info SET name = ? WHERE uid = ?", req.GetName(), ctx.Uid()); err != nil {
			logger.Errorf("failed to update role info | %v", err)
			return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "update_role", err)
		}
		return rsp, nil
	}

	ctx.Infof("role not exist")
	if _, err := engine.Exec("INSERT INTO role_info VALUES (?, ?)", ctx.Uid(), req.GetName()); err != nil {
		logger.Errorf("failed to insert role info | %v", err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "insert_role", err)
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) SearchRole(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerSearchRoleReq) (*g1_protocol.MysqlInnerSearchRoleRsp, error) {
	rsp := &g1_protocol.MysqlInnerSearchRoleRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	engine := globals.OrmMgr.GetOrmEngine()
	if engine == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}

	results, err := engine.QueryString("SELECT uid FROM role_info WHERE name = ?", req.GetSearchString())
	if err != nil {
		logger.Errorf("failed to select role info: %v", err)
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_FAIL, "biz_error", "")
	}

	for _, row := range results {
		uid, parseErr := strconv.ParseUint(row["uid"], 10, 64)
		if parseErr != nil {
			logger.Errorf("scan error: %v", parseErr)
			continue
		}
		rsp.Uid = uid
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) Update(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerUpdateReq) (*emptypb.Empty, error) {
	switch req.GetDataType() {
	case g1_protocol.DataType_DATA_TYPE_TEXAS_ROOM_INFO:
		manager.Push(int64(req.GetId()), saveRoomInfo(req.GetData()))
	case g1_protocol.DataType_DATA_TYPE_TEXAS_GAME_RECORD:
		manager.Push(int64(req.GetId()), saveGameInfo(req.GetData()))
	case g1_protocol.DataType_DATA_TYPE_PLAYER_INFO:
		manager.Push(int64(req.GetId()), savePlayerInfo(req.GetData()))
	}
	return &emptypb.Empty{}, nil
}

func (s *MysqlServiceImpl) QueryRoomInfo(ctx *ssrpc.Context, req *g1_protocol.QueryRoomInfoReq) (*g1_protocol.QueryRoomInfoRsp, error) {
	session := globals.OrmMgr.GetOrmEngine().NewSession()
	defer session.Close()

	session.Where("room_id = ?", req.GetRoomId())
	if req.GetTableId() > 0 {
		session.And("table_id = ?", req.GetTableId())
	}
	if req.GetGameType() > 0 {
		session.And("game_type = ?", req.GetGameType())
	}
	if req.GetRoomStage() > 0 {
		session.And("room_stage = ?", req.GetRoomStage())
	}
	if req.GetBlind() != "" {
		session.And("blind = ?", req.GetBlind())
	}
	if req.GetBeginTime() > 0 {
		session.And("create_time >= ?", req.GetBeginTime())
	}
	if req.GetEndTime() > 0 {
		session.And("finish_time <= ?", req.GetEndTime())
	}

	rsp := &g1_protocol.QueryRoomInfoRsp{List: []*g1_protocol.MysqlTexasRoomInfo{}}
	if err := session.Find(&rsp.List); err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query room info failed")
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) QueryPlayerInfo(ctx *ssrpc.Context, req *g1_protocol.QueryPlayerInfoReq) (*g1_protocol.QueryPlayerInfoRsp, error) {
	session := globals.OrmMgr.GetOrmEngine().NewSession()
	defer session.Close()

	session.Where("uid = ?", req.GetUid())
	if req.GetTableId() > 0 {
		session.And("table_id = ?", req.GetTableId())
	}
	if req.GetRoomId() > 0 {
		session.And("room_id = ?", req.GetRoomId())
	}
	if req.GetGameType() > 0 {
		session.And("game_type = ?", req.GetGameType())
	}
	if req.GetRoomStage() > 0 {
		session.And("room_stage = ?", req.GetRoomStage())
	}
	if req.GetBlind() != "" {
		session.And("blind = ?", req.GetBlind())
	}
	if req.GetBeginTime() > 0 {
		session.And("begin_time >= ?", req.GetBeginTime())
	}
	if req.GetEndTime() > 0 {
		session.And("end_time <= ?", req.GetEndTime())
	}

	rsp := &g1_protocol.QueryPlayerInfoRsp{List: []*g1_protocol.MysqlTexasPlayerInfo{}}
	if err := session.Find(&rsp.List); err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query player info failed")
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) QueryGameInfo(ctx *ssrpc.Context, req *g1_protocol.QueryGameInfoReq) (*g1_protocol.QueryGameInfoRsp, error) {
	cli := globals.OrmMgr.GetOrmEngine()
	item := &g1_protocol.MysqlTexasGameInfo{GameId: req.GetGameId()}
	ok, err := cli.Get(item)
	if err != nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "query game info failed")
	}

	rsp := &g1_protocol.QueryGameInfoRsp{}
	if ok {
		detail := &g1_protocol.TexasGameRecordDetail{}
		_ = proto.Unmarshal(item.GameDetail, detail)
		rsp.Data = &g1_protocol.TexasGameRecord{
			TableId:      item.TableId,
			GameType:     item.GameType,
			RoomStage:    item.RoomStage,
			Blind:        item.Blind,
			BeginTime:    item.BeginTime,
			EndTime:      item.EndTime,
			TotalPot:     item.TotalPot,
			TotalService: item.TotalService,
			Detail:       detail,
			Round:        item.Round,
		}
	}
	return rsp, nil
}

func mysqlRoleExists(ctx *ssrpc.Context) bool {
	engine := globals.OrmMgr.GetOrmEngine()
	if engine == nil {
		return false
	}

	results, err := engine.QueryString("SELECT uid FROM role_info WHERE uid = ?", ctx.Uid())
	if err != nil {
		logger.Errorf("failed to check role exist | %v", err)
		return false
	}
	return len(results) > 0
}

func saveRoomInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasRoomInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("data unmarshal failed: %v", err)
			return
		}

		cli := globals.OrmMgr.GetOrmEngine()
		old := &g1_protocol.MysqlTexasRoomInfo{RoomId: item.RoomId, TableId: item.TableId}
		ok, err := cli.Get(old)
		if err != nil {
			logger.Errorf("query room info failed: %v", err)
			return
		}
		if !ok {
			if _, err := cli.InsertOne(item); err != nil {
				logger.Errorf("insert room info failed: %v", err)
			}
			return
		}
		if old.UpdateTime > item.UpdateTime {
			logger.Errorf("stale room info. new=%v old=%v", item, old)
			return
		}
		if _, err := cli.Update(item, old); err != nil {
			logger.Errorf("update room info failed: %v", err)
		}
	}
}

func saveGameInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasGameInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("data unmarshal failed: %v", err)
			return
		}

		cli := globals.OrmMgr.GetOrmEngine()
		old := &g1_protocol.MysqlTexasGameInfo{GameId: item.GameId}
		ok, err := cli.Get(old)
		if err != nil {
			logger.Errorf("query game info failed: %v", err)
			return
		}
		if !ok {
			if _, err := cli.InsertOne(item); err != nil {
				logger.Errorf("insert game info failed: %v", err)
			}
			return
		}
		if old.UpdateTime > item.UpdateTime {
			logger.Errorf("stale game info. new=%v old=%v", item, old)
			return
		}
		if _, err := cli.Update(item, old); err != nil {
			logger.Errorf("update game info failed: %v", err)
		}
	}
}

func savePlayerInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasPlayerInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("data unmarshal failed: %v", err)
			return
		}
		if _, err := globals.OrmMgr.GetOrmEngine().InsertOne(item); err != nil {
			logger.Errorf("insert player info failed: %v", err)
		}
	}
}
