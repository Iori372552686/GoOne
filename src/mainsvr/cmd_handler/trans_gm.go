package cmd_handler

import (
	"GoOne/lib/api/cmd_handler"
	"GoOne/lib/api/logger"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/mainsvr/globals"
	"GoOne/src/mainsvr/role"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

type GmGetRole struct{}

func (h *GmGetRole) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	ret := 0
	rsp := g1_protocol.GMGetRoleRsp{}
	for {
		myRole := globals.RoleMgr.GetOrLoadRole(c.Uid(), c)
		if myRole == nil {
			c.Infof("Gm try to get not existing role.")
			ret = -1
			break
		}

		// send rsp
		rsp.RoleInfo = new(g1_protocol.RoleInfo)
		proto.Merge(rsp.RoleInfo, myRole.PbRole)

		glog.Infof("GM get role {uid: %d}", c.Uid())
		break
	}

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(&rsp)
	return ret
}

type GmSetRole struct{}

func (t *GmSetRole) ProcessCmd(c cmd_handler.IContext, data []byte, myRole *role.Role) int {
	req := &g1_protocol.GMSetRoleReq{}
	rsp := &g1_protocol.GMSetRoleRsp{}

	c.Infof("GMSetRoleReq: %v", req)
	err := c.ParseMsg(data, req)
	if err != nil || req.RoleInfo == nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	// 先保存connbus
	connBus := myRole.PbRole.ConnSvrInfo.BusId

	myRole.PbRole = req.RoleInfo

	if myRole.PbRole.ConnSvrInfo == nil {
		myRole.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	myRole.PbRole.ConnSvrInfo.BusId = connBus
	logger.Errorf("%v", myRole.PbRole.String())

	_ = myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_ALL)

	rsp.Ret = &g1_protocol.Ret{Ret: 0}
	c.SendMsgBack(rsp)

	return 0
}

type GmAddItem struct{}

func (t *GmAddItem) ProcessCmd(c cmd_handler.IContext, data []byte, myRole *role.Role) int {
	req := &g1_protocol.GMAddItemReq{}
	rsp := &g1_protocol.GMAddItemRsp{}
	c.Infof("GMAddItemReq: %v", req)
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := myRole.ItemAdd(req.Id, req.Count,
		&role.Reason{role.REASON_GM, 0})

	_ = myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_ALL)

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(rsp)

	return 0
}
