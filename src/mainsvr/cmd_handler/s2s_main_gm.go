package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
)

func GmGetRole(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	ret := g1_protocol.ErrorCode_ERR_OK
	rsp := g1_protocol.GMGetRoleRsp{}
	for {
		myRole := globals.RoleMgr.GetOrLoadRole(c.Uid(), c)
		if myRole == nil {
			c.Infof("Gm try to get not existing role.")
			ret = g1_protocol.ErrorCode_ERR_DB
			break
		}

		// send rsp
		rsp.RoleInfo = new(g1_protocol.RoleInfo)
		proto.Merge(rsp.RoleInfo, myRole.PbRole)
		logger.Infof("GM get role {uid: %d}", c.Uid())
		break
	}

	rsp.Ret = &g1_protocol.Ret{Code: ret}
	c.SendMsgBack(&rsp)
	return ret
}

func GmSetRole(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GMSetRoleReq{}
	rsp := &g1_protocol.GMSetRoleRsp{}

	c.Infof("GMSetRoleReq: %v", req)
	err := c.ParseMsg(data, req)
	if err != nil || req.RoleInfo == nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	// 先保存connbus
	connBus := myRole.PbRole.ConnSvrInfo.BusId

	myRole.PbRole = req.RoleInfo

	if myRole.PbRole.ConnSvrInfo == nil {
		myRole.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	myRole.PbRole.ConnSvrInfo.BusId = connBus
	logger.Errorf("%v", myRole.PbRole.String())
	myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_ALL)
	rsp.Ret = &g1_protocol.Ret{Code: 0}
	c.SendMsgBack(rsp)

	return g1_protocol.ErrorCode_ERR_OK
}

func GmAddItem(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.GMAddItemReq{}
	rsp := &g1_protocol.GMAddItemRsp{}
	c.Infof("GMAddItemReq: %v", req)
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := myRole.ItemAdd(req.Id, req.Count, &role.Reason{g1_protocol.Reason_REASON_GM, 0})
	myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_ALL)
	rsp.Ret = &g1_protocol.Ret{Code: ret}
	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode_ERR_OK
}
