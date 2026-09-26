package service

import (
	"strconv"

	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/golang/protobuf/proto"
)

// ===== 登录 / 退出 / 心跳 =====

func (s *MainC2SServiceImpl) Login(ctx *ssrpc.Context, req *g1_protocol.LoginReq) (*g1_protocol.LoginRsp, error) {
	// uid 已由 connsvr 网关认证绑定（见 src/connsvr/pack_proc.go），此处只记录
	// 账号信息；凭证校验不在本服务重复执行。
	ctx.Infof("---------------  Login  %d  account=%s channel=%d login_type=%s  ---------------",
		ctx.Uid(), req.GetAccount(), req.GetChannelId(), req.GetLoginType())

	rsp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	myRole := s.roles.LoadOrCreate(ctx.Uid(), ctx)
	if myRole == nil {
		ctx.Errorf("Failed to get role. {req:%v}", req)
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER, "biz_error", "")
	}

	processConnSvrInfo(ctx, myRole)
	now := myRole.Now()
	myRole.OnLogin(now)
	// 时间是0 就没有上次登录的时间
	myRole.PbRole.LoginInfo.LastLoginTime = myRole.PbRole.LoginInfo.NowLoginTime
	myRole.PbRole.LoginInfo.NowLoginTime = now
	myRole.OnClientHeartbeat(now)
	myRole.AfterLogin(now)

	// rsp
	rsp.TimeNowMs = myRole.NowMs()
	rsp.RoleInfo = new(g1_protocol.RoleInfo)
	proto.Merge(rsp.RoleInfo, myRole.PbRole)
	ctx.Infof("role login {uid:%d, role_size:%d}", ctx.Uid(), proto.Size(rsp.RoleInfo))
	_ = myRole.FlushPending(ctx, false)

	return rsp, nil
}

func (s *MainC2SServiceImpl) Logout(ctx *ssrpc.Context, req *g1_protocol.LogoutReq) (*g1_protocol.LogoutRsp, error) {
	// 保存并退出的完整契约（保存失败保留待重试、心跳过期守卫、幂等删除）收敛在
	// RoleMgr.Logout 用例内（role/session.go，F04）；此处只做协议转换。
	if err := s.roles.Logout(ctx.Uid(), ctx, req.GetByServer(), req.GetReason()); err != nil {
		return &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{
			Code: g1_protocol.ErrorCode_ERR_DB,
			Msg:  "logout incomplete: save failed, will retry in background",
		}}, nil
	}

	// If server-triggered logout, legacy behavior: no rsp.
	if req.GetByServer() {
		ctx.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", ctx.Uid(), req.GetByServer(), req.GetReason())
		return nil, nil
	}

	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	ctx.Infof("role logout{uid: %d, ByServer: %v, Reason: %v}", ctx.Uid(), req.GetByServer(), req.GetReason())
	return rsp, nil
}

func (s *MainC2SServiceImpl) HeartBeat(ctx *ssrpc.Context, req *g1_protocol.HeartBeatReq) (*g1_protocol.HeartBeatRsp, error) {
	myRole := s.roles.Load(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	ret := g1_protocol.ErrorCode_ERR_OK
	now := myRole.Now()
	myRole.OnClientHeartbeat(now)
	_ = myRole.FlushPending(ctx, false)

	rsp := &g1_protocol.HeartBeatRsp{
		ClientNowMsInReq: req.GetClientNowMs(),
		ServerNowMs:      myRole.NowMs(),
		Ret:              &g1_protocol.Ret{Code: ret},
	}
	return rsp, nil
}

// processConnSvrInfo 把网关连接信息（connsvr BusId + 客户端 ip:port）记录到角色，
// 供后续定向推送/踢人使用。
func processConnSvrInfo(c interface {
	Uid() uint64
	OriSrcBusId() uint32
	Ip() uint32
	Flag() uint32
}, myRole *role.Role) {
	connSvrInfo := myRole.PbRole.ConnSvrInfo

	ipStr := bus.IpIntToString(c.Ip())
	portStr := strconv.Itoa(int(c.Flag())) // 端口是存在flag字段里面的
	remoteAddr := ipStr + ":" + portStr

	connSvrInfo.BusId = c.OriSrcBusId()
	connSvrInfo.ClientPos = remoteAddr
}
