package service

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ===== GM 运维 =====

func (s *MainC2SServiceImpl) GmGetRole(ctx *ssrpc.Context, req *g1_protocol.GMGetRoleReq) (*g1_protocol.GMGetRoleRsp, error) {
	_ = req
	ret := g1_protocol.ErrorCode_ERR_OK
	rsp := &g1_protocol.GMGetRoleRsp{}

	myRole := s.roles.Load(ctx.Uid(), ctx)
	if myRole == nil {
		ctx.Infof("Gm try to get not existing role.")
		ret = g1_protocol.ErrorCode_ERR_DB
		rsp.Ret = &g1_protocol.Ret{Code: ret}
		return rsp, nil
	}

	rsp.RoleInfo = new(g1_protocol.RoleInfo)
	proto.Merge(rsp.RoleInfo, myRole.PbRole)
	logger.Infof("GM get role {uid: %d}", ctx.Uid())

	rsp.Ret = &g1_protocol.Ret{Code: ret}
	return rsp, nil
}

func (s *MainC2SServiceImpl) GmSetRole(ctx *ssrpc.Context, req *g1_protocol.GMSetRoleReq) (*g1_protocol.GMSetRoleRsp, error) {
	myRole := s.roles.Load(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	if req.GetRoleInfo() == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_MARSHAL, "missing role_info")
	}

	// 先保存 connbus
	connBus := uint32(0)
	if myRole.PbRole != nil && myRole.PbRole.ConnSvrInfo != nil {
		connBus = myRole.PbRole.ConnSvrInfo.BusId
	}

	myRole.PbRole = req.GetRoleInfo()
	if myRole.PbRole.ConnSvrInfo == nil {
		myRole.PbRole.ConnSvrInfo = &g1_protocol.ConnSvrInfo{}
	}
	myRole.PbRole.ConnSvrInfo.BusId = connBus

	logger.Infof("GM set role {uid:%d, role_size:%d}", ctx.Uid(), proto.Size(myRole.PbRole))
	myRole.MarkFullSync(g1_protocol.ERoleSectionFlag_ALL)
	myRole.MarkPersistDirty("gm_set_role")
	_ = myRole.FlushPending(ctx, true)

	return &g1_protocol.GMSetRoleRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}, nil
}

func (s *MainC2SServiceImpl) GmAddItem(ctx *ssrpc.Context, req *g1_protocol.GMAddItemReq) (*g1_protocol.GMAddItemRsp, error) {
	myRole := s.roles.Load(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	ret := myRole.ItemAdd(req.GetId(), req.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_GM, Scene: 0})
	_ = myRole.FlushPending(ctx, false)
	return &g1_protocol.GMAddItemRsp{Ret: &g1_protocol.Ret{Code: ret}}, nil
}

func (s *MainC2SServiceImpl) BatchAddItem(ctx *ssrpc.Context, req *g1_protocol.BatchAddItemReq) (*g1_protocol.BatchAddItemRsp, error) {
	myRole := s.roles.Load(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	code := g1_protocol.ErrorCode_ERR_OK
	for _, it := range req.GetItems() {
		if ret := myRole.ItemAdd(it.GetId(), it.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_GM, Scene: 0}); ret != g1_protocol.ErrorCode_ERR_OK {
			code = ret
		}
	}
	_ = myRole.FlushPending(ctx, false)
	return &g1_protocol.BatchAddItemRsp{Ret: &g1_protocol.Ret{Code: code}}, nil
}
