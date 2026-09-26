package service

import (
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// ===== 背包 / 道具 =====

func (s *MainC2SServiceImpl) UseItem(ctx *ssrpc.Context, req *g1_protocol.UseItemReq) (*g1_protocol.UseItemRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	ret := myRole.ItemUse(req.GetItemId(), req.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_CONSUME, Scene: 0})
	_ = myRole.FlushPending(ctx, false)
	return &g1_protocol.UseItemRsp{Ret: &g1_protocol.Ret{Code: ret}}, nil
}

func (s *MainC2SServiceImpl) SellItem(ctx *ssrpc.Context, req *g1_protocol.SellItemReq) (*g1_protocol.SellItemRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	ret := myRole.ItemSell(req.GetItemId(), req.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_CONSUME, Scene: 0})
	_ = myRole.FlushPending(ctx, false)
	return &g1_protocol.SellItemRsp{Ret: &g1_protocol.Ret{Code: ret}}, nil
}

func (s *MainC2SServiceImpl) DecomposeItem(ctx *ssrpc.Context, req *g1_protocol.DecomposeItemReq) (*g1_protocol.DecomposeItemRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	rewards, ret := myRole.ItemDecompose(req.GetItemId(), req.GetCount(), &role.Reason{Reason: g1_protocol.Reason_REASON_CONSUME, Scene: 0})
	_ = myRole.FlushPending(ctx, false)
	// 分解产出触发展示
	if rewards != nil && len(*rewards) > 0 {
		_ = myRole.ObtainNotifyItems("decompose", *rewards)
	}
	rsp := &g1_protocol.DecomposeItemRsp{Ret: &g1_protocol.Ret{Code: ret}}
	if rewards != nil {
		rsp.Rewards = *rewards
	}
	return rsp, nil
}

func (s *MainC2SServiceImpl) QueryBackpack(ctx *ssrpc.Context, req *g1_protocol.QueryBackpackReq) (*g1_protocol.QueryBackpackRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}
	total, items := myRole.QueryBackpack(req.GetBagType(), req.GetPageIdx(), req.GetPageSize())
	return &g1_protocol.QueryBackpackRsp{
		Ret:      &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK},
		Total:    total,
		PageIdx:  req.GetPageIdx(),
		PageSize: req.GetPageSize(),
		BagType:  req.GetBagType(),
		Items:    items,
	}, nil
}
