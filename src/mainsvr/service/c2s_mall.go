package service

import (
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/gamedata/repository/mall"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// ===== 商城 =====

func (s *MainC2SServiceImpl) MallBuyPackage(ctx *ssrpc.Context, req *g1_protocol.MallBuyPackageReq) (*g1_protocol.MallBuyPackageRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.MallBuyPackageRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	code := myRole.MallCheckBuyCondition(req.GetConfId())
	if code != g1_protocol.ErrorCode_ERR_OK {
		return rsp, gerr.New(code, "mall_check", "")
	}

	conf := mall.GetMallById(req.GetConfId())
	if conf == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_CONF, "conf_not_found", "")
	}

	_, code = myRole.ItemCheckReduce(conf.CostItemID, int64(conf.CostItemCnt))
	if code != g1_protocol.ErrorCode_ERR_OK {
		return rsp, gerr.New(code, "item_check_reduce", "")
	}

	// 如果是充值购买的礼包就走充值（暂未实现，保持旧逻辑）
	if int32(g1_protocol.EItemID_ACECOIN) == conf.CostItemID {
		// ret = RechargeAdd(conf.Rmb, myRole)
	} else {
		code = myRole.ItemExchange(conf.CostItemID, int64(conf.CostItemCnt), conf.PackageID,
			1, &role.Reason{Reason: g1_protocol.Reason_REASON_MALL_PACKAGE, Scene: req.GetConfId()})
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "item_exchange", "")
		}
	}

	myRole.MallAddBuyCount(req.GetConfId())
	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}
