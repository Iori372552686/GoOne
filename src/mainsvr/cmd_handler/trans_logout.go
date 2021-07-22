package cmd_handler

import (
	`GoOne/lib/cmd_handler`
	g1_protocol `GoOne/protobuf/protocol`
	`GoOne/src/mainsvr/globals`
)

type Logout struct {}
func (h *Logout) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.LogoutReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0

	myRole := globals.RoleMgr.GetRole(c.Uid())
	if myRole == nil {
		c.Debugf("Already logged out. {req=%#v}", req)
		return 0
	}

	myRole.PbRole.LoginInfo.LastLogoutTime = myRole.Now()
	// 考虑到如果mainsvr重启，玩家数据需要从db拉，这里不需要将connsvr的busID置为0
	//myRole.PbRole.ConnSvrInfo.BusId = 0
	myRole.SaveToDB(c)
	// 记录登出信息
	//todo 启动一个单独的协程进行处理,注意先后顺序
	//clogmgr.Logout(myRole)  //todo: uncomment this
	globals.RoleMgr.DeleteRole(c.Uid())

	if !req.ByServer { // 客户端需要回包
		rsp := g1_protocol.LogoutRsp{}
		rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
		c.SendMsgBack(&rsp)
	}

	c.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", c.Uid(), req.ByServer, req.Reason)

	return ret
}
