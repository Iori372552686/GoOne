package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/router"
	g1_protocol "github.com/Iori372552686/GoOne/protobuf/protocol"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	"github.com/golang/protobuf/proto"

	"strconv"
)

type Login struct{}

func (h *Login) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.LoginReq{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	c.Infof("Req: %v", req)

	ret := 0
	rsp := g1_protocol.LoginRsp{}
	for {
		myRole := globals.RoleMgr.GetOrLoadOrCreateRole(c.Uid(), c)
		if myRole == nil {
			c.Errorf("Failed to get role. {req:%v}", req)
			ret = -1
			break
		}

		h.processConnSvrInfo(c, myRole)

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

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(&rsp)
	return ret
}

func (h *Login) processConnSvrInfo(c cmd_handler.IContext, myRole *role.Role) {
	connSvrInfo := myRole.PbRole.ConnSvrInfo

	ipStr := bus.IpIntToString(c.Ip())
	portStr := strconv.Itoa(int(c.Flag())) // 端口是存在flag字段里面的

	remoteAddr := ipStr + ":" + portStr
	if connSvrInfo.ClientPos != "" && connSvrInfo.ClientPos != remoteAddr {
		c.Infof("Kick multi-place login")
		req := g1_protocol.ConnKickOutReq{}
		req.Reason = g1_protocol.EKickOutReason_MULTI_PLACE_LOGIN
		req.RemoteAddr = connSvrInfo.ClientPos
		_ = router.SendPbMsgByBusIdSimple(connSvrInfo.BusId,
			c.Uid(),
			uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ),
			&req)
	}
	connSvrInfo.BusId = c.OriSrcBusId()
	connSvrInfo.ClientPos = remoteAddr

}
