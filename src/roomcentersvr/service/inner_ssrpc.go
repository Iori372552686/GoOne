package service

import (
	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// RoomCenterInnerServiceImpl 是 roomcentersvr 内部命令（s2s）的 IDL 驱动 ssrpc 实现。
// 业务语义全部收敛在 TexasRoomCenterMgr 的带锁封装方法内
// （room_mgr/texas_room/base.go：数据+行为+锁一体），本层只做
// "zone 路由分片定位 + 委托 + 错误码转换"，不触碰任何共享数据。
type RoomCenterInnerServiceImpl struct {
	roomcenterv1.RoomCenterInnerServiceSS
}

func (s *RoomCenterInnerServiceImpl) Tick(ctx *ssrpc.Context, req *g1_protocol.InnerTickReq) (*emptypb.Empty, error) {
	// routerId（Rid）即 zone 路由分片串行键：同 zone 的 tick 与业务请求在
	// TransMgr 内串行执行， mgr 内部锁再兜底跨串行键来源（gamesvr 上报/持久化）。
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}

	ins.Tick(req.GetNowMs())
	return nil, nil // one-way
}

func (s *RoomCenterInnerServiceImpl) RoomList(ctx *ssrpc.Context, req *g1_protocol.RoomListReq) (*g1_protocol.RoomListRsp, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	return ins.RoomListPage(req), nil
}

func (s *RoomCenterInnerServiceImpl) QuickStart(ctx *ssrpc.Context, req *g1_protocol.QuickStartReq) (*g1_protocol.QuickStartRsp, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	return ins.QuickStart(req), nil
}

// QuickStartRollback 快速开始占位回滚（mainsvr 加入对局失败时回调，one-way）。
// 幂等安全：房间不存在视为已归还，计数只减到 0（详见 mgr 实现）。
func (s *RoomCenterInnerServiceImpl) QuickStartRollback(ctx *ssrpc.Context, req *g1_protocol.QuickStartRollbackReq) (*emptypb.Empty, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	if code := ins.QuickStartRollback(req); code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.E(code, code.String())
	}
	return nil, nil
}

// UpdateRoomInfo gamesvr 房间信息上报（one-way，权威数据源，整房替换）。
func (s *RoomCenterInnerServiceImpl) UpdateRoomInfo(ctx *ssrpc.Context, req *g1_protocol.RoomShowInfo) (*emptypb.Empty, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	if code := ins.UpdateRoomInfo(req); code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.E(code, code.String())
	}
	return nil, nil
}

// DelRoomInfo gamesvr 房间解散上报（one-way，幂等删除）。
func (s *RoomCenterInnerServiceImpl) DelRoomInfo(ctx *ssrpc.Context, req *g1_protocol.RoomShowInfo) (*emptypb.Empty, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	if code := ins.DelRoomInfo(req); code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.E(code, code.String())
	}
	return nil, nil
}
