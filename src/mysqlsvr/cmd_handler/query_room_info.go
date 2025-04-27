package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

func QueryRoomInfoRequest(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.QueryRoomInfoReq{}
	rsp := &g1_protocol.QueryRoomInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_MARSHAL
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	// 查询房间信息
	session := globals.OrmMgr.GetOrmEngine().NewSession()
	defer session.Close()

	session.Where("room_id = ?", req.RoomId)
	if req.TableId > 0 {
		session.And("table_id = ?", req.TableId)
	}
	if req.GameType > 0 {
		session.And("game_type = ?", req.GameType)
	}
	if req.RoomStage > 0 {
		session.And("room_stage = ?", req.RoomStage)
	}
	if len(req.Blind) > 0 {
		session.And("blind = ?", req.Blind)
	}
	if req.BeginTime > 0 {
		session.And("create_time >= ?", req.BeginTime)
	}
	if req.EndTime > 0 {
		session.And("finish_time <= ?", req.EndTime)
	}
	// 查询房间列表
	rsp.List = []*g1_protocol.MysqlTexasRoomInfo{}
	if err := session.Find(&rsp.List); err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_DB
		return g1_protocol.ErrorCode_ERR_DB
	}
	// 返回
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func QueryPlayerInfoRequest(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.QueryPlayerInfoReq{}
	rsp := &g1_protocol.QueryPlayerInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_MARSHAL
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	// 查询房间信息
	session := globals.OrmMgr.GetOrmEngine().NewSession()
	defer session.Close()
	session.Where("uid = ?", req.Uid)
	if req.TableId > 0 {
		session.And("table_id = ?", req.TableId)
	}
	if req.RoomId > 0 {
		session.And("room_id = ?", req.RoomId)
	}
	if req.GameType > 0 {
		session.And("game_type = ?", req.GameType)
	}
	if req.RoomStage > 0 {
		session.And("room_stage = ?", req.RoomStage)
	}
	if len(req.Blind) > 0 {
		session.And("blind = ?", req.Blind)
	}
	if req.BeginTime > 0 {
		session.And("begin_time >= ?", req.BeginTime)
	}
	if req.EndTime > 0 {
		session.And("end_time <= ?", req.EndTime)
	}
	// 查询房间列表
	rsp.List = []*g1_protocol.MysqlTexasPlayerInfo{}
	if err := session.Find(&rsp.List); err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_DB
		return g1_protocol.ErrorCode_ERR_DB
	}
	// 返回
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}

func QueryGameInfoRequest(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.QueryGameInfoReq{}
	rsp := &g1_protocol.QueryGameInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_MARSHAL
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	// 查询游戏信息
	cli := globals.OrmMgr.GetOrmEngine()
	item := &g1_protocol.MysqlTexasGameInfo{GameId: req.GameId}
	ok, err := cli.Get(item)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_DB
		return g1_protocol.ErrorCode_ERR_DB
	}
	if ok {
		detail := &g1_protocol.TexasGameRecordDetail{}
		proto.Unmarshal(item.GameDetail, detail)
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
	// 返回数据
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}
