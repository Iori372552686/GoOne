package gfunc

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/router"
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

func marshelData(op g1_protocol.GameNotifyType, pbMsg proto.Message) (*g1_protocol.GameUserEventNotify, error) {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return nil, err
	}

	notify := &g1_protocol.GameUserEventNotify{
		Event:   op,
		Content: data,
	}

	return notify, nil
}

// send single notify
func sendSingleNotify(busId uint32, uid uint64, data *g1_protocol.GameUserEventNotify) error {
	return router.SendPbMsgByBusIdSimple(busId, uid, g1_protocol.CMD_SC_GAME_EVENT_NOTIFY, data)
}

// -------------------------------  public  --------------------------------
// send game evnet notify by brief info
func SendGameEventNotifyBybrief(uidMap *map[uint64]*g1_protocol.PbRoleBriefInfo, op g1_protocol.GameNotifyType, pbMsg proto.Message) g1_protocol.ErrorCode {
	notify, err := marshelData(op, pbMsg)
	if err != nil {
		logger.Errorf("marshelData, op:%d, error: %s", op, err)
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	for uid, role := range *uidMap {
		err = sendSingleNotify(role.ConnBusId, uid, notify)
		if err != nil {
			logger.Errorf("SendGameEventNotifyBybrief, op:%d, uid:%s, data:%+v, error: %s", op, uid, pbMsg, err)
		}
	}

	return g1_protocol.ErrorCode_ERR_OK
}

// send game evnet notify by conn busid
func SendGameEventNotifyByConnBus(connBus *map[uint64]uint32, op g1_protocol.GameNotifyType, pbMsg proto.Message) g1_protocol.ErrorCode {
	notify, err := marshelData(op, pbMsg)
	if err != nil {
		logger.Errorf("marshelData, op:%v, error: %s", op, err)
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	for uid, busId := range *connBus {
		err = sendSingleNotify(busId, uid, notify)
		if err != nil {
			logger.Errorf("SendGameEventNotifyByConnBus, op:%v, uid:%v, data:%+v, error: %s", op, uid, pbMsg, err)
		}
	}

	return g1_protocol.ErrorCode_ERR_OK
}

// send game evnet notify to single
func SendRoleOpEventNotifyToSingle(busId uint32, uid uint64, op g1_protocol.GameNotifyType, pbMsg proto.Message) g1_protocol.ErrorCode {
	if busId == 0 || uid == 0 || op == 0 || pbMsg == nil {
		logger.Errorf("SendRoleOpEventNotifyToSingle, invalid params, busId:%d, uid:%d, cmd:%d, pbMsg:%+v", busId, uid, op, pbMsg)
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	notify, err := marshelData(op, pbMsg)
	if err != nil {
		logger.Errorf("marshelData, op:%d, error: %s", op, err)
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	err = sendSingleNotify(busId, uid, notify)
	if err != nil {
		logger.Errorf("sendSingleNotify, op:%d, uid:%s, data:%+v, error: %s", op, uid, pbMsg, err)
		return g1_protocol.ErrorCode_ERR_FAIL
	}

	return g1_protocol.ErrorCode_ERR_OK
}
