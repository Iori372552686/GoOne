package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

func UpdateRequest(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.MysqlInnerUpdateReq{}
	rsp := &g1_protocol.MysqlInnerUpdateRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_MARSHAL
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	switch req.DataType {
	case g1_protocol.DataType_DATA_TYPE_TEXAS_ROOM_INFO:
		manager.Push(int64(req.Id), saveRoomInfo(req.Data))

	case g1_protocol.DataType_DATA_TYPE_TEXAS_GAME_RECORD:
		manager.Push(int64(req.Id), saveGameInfo(req.Data))

	case g1_protocol.DataType_DATA_TYPE_PLAYER_INFO:
		manager.Push(int64(req.Id), savePlayerInfo(req.Data))
	}
	return g1_protocol.ErrorCode_ERR_OK
}

func saveRoomInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasRoomInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("数据解析失败: %v", string(buf))
		}
		// 读取数据
		cli := globals.OrmMgr.GetOrmEngine()
		old := &g1_protocol.MysqlTexasRoomInfo{RoomId: item.RoomId, TableId: item.TableId}
		ok, err := cli.Get(old)
		if err != nil {
			logger.Errorf("数据查询失败: %v", err)
			return
		}
		if !ok {
			if _, err := cli.InsertOne(item); err != nil {
				logger.Errorf("数据插入失败: %v", err)
			}
			return
		}
		if old.UpdateTime > item.UpdateTime {
			logger.Errorf("数据已经过期. new: %v, old: %v", old, item)
			return
		}
		if _, err := cli.Update(item, old); err != nil {
			logger.Errorf("数据更新失败: %v", err)
			return
		}
	}
}

func saveGameInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasGameInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("数据解析失败: %v", string(buf))
		}
		// 读取数据
		cli := globals.OrmMgr.GetOrmEngine()
		old := &g1_protocol.MysqlTexasGameInfo{GameId: item.GameId}
		ok, err := cli.Get(old)
		if err != nil {
			logger.Errorf("数据查询失败: %v", err)
			return
		}
		if !ok {
			if _, err := cli.InsertOne(item); err != nil {
				logger.Errorf("数据插入失败: %v", err)
			}
			return
		}
		// 插入或者更新数据
		if old.UpdateTime > item.UpdateTime {
			logger.Errorf("数据已经过期. new: %v, old: %v", old, item)
			return
		}
		if _, err := cli.Update(item, old); err != nil {
			logger.Errorf("数据更新失败: %v", err)
			return
		}
	}
}

func savePlayerInfo(buf []byte) func() {
	return func() {
		item := &g1_protocol.MysqlTexasPlayerInfo{}
		if err := proto.Unmarshal(buf, item); err != nil {
			logger.Errorf("数据解析失败: %v", string(buf))
		}
		// 读取数据
		cli := globals.OrmMgr.GetOrmEngine()
		if _, err := cli.InsertOne(item); err != nil {
			logger.Errorf("数据插入失败: %v", err)
			return
		}
	}
}
