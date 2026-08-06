package service

import (
	roomcenterv1 "github.com/Iori372552686/GoOne/api/gen/game/roomcenter/v1"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/logic"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// RoomCenterInnerServiceImpl is the IDL-driven ssrpc implementation for roomcentersvr internal commands.
type RoomCenterInnerServiceImpl struct {
	roomcenterv1.RoomCenterInnerServiceSS
}

func (s *RoomCenterInnerServiceImpl) Tick(ctx *ssrpc.Context, req *g1_protocol.InnerTickReq) (*emptypb.Empty, error) {
	// Keep the exact same routing semantics as the legacy adapter:
	// routerId (Rid) selects the zone manager.
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		// legacy code returned ERR_ARGV; keep behavior.
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
	return logic.OnCenterRoomList(req, ins), nil
}

func (s *RoomCenterInnerServiceImpl) QuickStart(ctx *ssrpc.Context, req *g1_protocol.QuickStartReq) (*g1_protocol.QuickStartRsp, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	return logic.OnCenterQuickStart(req, ins), nil
}

func (s *RoomCenterInnerServiceImpl) UpdateRoomInfo(ctx *ssrpc.Context, req *g1_protocol.RoomShowInfo) (*emptypb.Empty, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	if code := logic.OnUpdateTexasRoomInfo(req, ins); code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.E(code, code.String())
	}
	return nil, nil
}

func (s *RoomCenterInnerServiceImpl) DelRoomInfo(ctx *ssrpc.Context, req *g1_protocol.RoomShowInfo) (*emptypb.Empty, error) {
	ins := globals.RoomListMgr.GetRoomMgrObj(ctx.Rid())
	if ins == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "room mgr not found")
	}
	if code := logic.OnDelTexasRoomInfo(req, ins); code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.E(code, code.String())
	}
	return nil, nil
}
