package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"

	"strconv"
)

func Login(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.LoginReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	c.Infof("---------------  Login  %d     ---------------", c.Uid())
	rsp := g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	for {
		myRole := globals.RoleMgr.GetOrLoadOrCreateRole(c.Uid(), c)
		if myRole == nil {
			c.Errorf("Failed to get role. {req:%v}", req)
			rsp.Ret.Code = g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER
			break
		}

		processConnSvrInfo(c, myRole)
		now := myRole.Now()
		myRole.OnLogin(now)
		//时间是0 就没有上次登录的时间
		myRole.PbRole.LoginInfo.LastLoginTime = myRole.PbRole.LoginInfo.NowLoginTime
		myRole.PbRole.LoginInfo.NowLoginTime = now
		myRole.OnClientHeartbeat(now)
		myRole.AfterLogin(now)

		// send rsp
		rsp.TimeNowMs = myRole.NowMs()
		rsp.RoleInfo = new(g1_protocol.RoleInfo)
		proto.Merge(rsp.RoleInfo, myRole.PbRole)

		c.Infof("role login, %v", rsp.RoleInfo.String())
		break
	}

	c.SendMsgBack(&rsp)
	return rsp.Ret.Code
}

func processConnSvrInfo(c cmd_handler.IContext, myRole *role.Role) {
	connSvrInfo := myRole.PbRole.ConnSvrInfo

	ipStr := bus.IpIntToString(c.Ip())
	portStr := strconv.Itoa(int(c.Flag())) // 端口是存在flag字段里面的

	remoteAddr := ipStr + ":" + portStr

	//交给conn 超时清理fd 这里不再重复
	/*	if connSvrInfo.ClientPos != "" && connSvrInfo.ClientPos != remoteAddr {
		c.Infof("Kick multi-place login")
		req := g1_protocol.ConnKickOutReq{}
		req.Reason = g1_protocol.EKickOutReason_MULTI_PLACE_LOGIN
		req.RemoteAddr = connSvrInfo.ClientPos
		router.SendPbMsgByBusIdSimple(connSvrInfo.BusId, c.Uid(), g1_protocol.CMD_CONN_KICK_OUT_REQ, &req)
	}*/

	connSvrInfo.BusId = c.OriSrcBusId()
	connSvrInfo.ClientPos = remoteAddr
}

func Logout(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.LogoutReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := g1_protocol.ErrorCode_ERR_OK
	myRole := globals.RoleMgr.GetRole(c.Uid())
	if myRole == nil {
		c.Debugf("Already logged out. {req=%#v}", req)
		return g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER
	}

	myRole.PbRole.LoginInfo.LastLogoutTime = myRole.Now()
	// 考虑到如果mainsvr重启，玩家数据需要从db拉，这里不需要将connsvr的busID置为0
	//myRole.PbRole.ConnSvrInfo.BusId = 0
	myRole.SaveToDB(c)
	// 记录登出信息
	//todo 启动一个单独的协程进行处理,注意先后顺序
	//clogmgr.Logout(myRole)
	globals.RoleMgr.DeleteRole(c.Uid())

	if !req.ByServer { // 客户端需要回包
		rsp := g1_protocol.LogoutRsp{}
		rsp.Ret = &g1_protocol.Ret{Code: ret}
		c.SendMsgBack(&rsp)
	}

	c.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", c.Uid(), req.ByServer, req.Reason)
	return ret
}

func HeartBeat(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.HeartBeatReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := g1_protocol.ErrorCode_ERR_OK
	now := myRole.Now()
	myRole.OnClientHeartbeat(now)

	rsp := &g1_protocol.HeartBeatRsp{}
	rsp.ClientNowMsInReq = req.ClientNowMs
	rsp.ServerNowMs = myRole.NowMs()
	rsp.Ret = &g1_protocol.Ret{Code: ret}
	c.SendMsgBack(rsp)

	return ret
}
