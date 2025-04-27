package cmd_handler

import (
	"github.com/Iori372552686/GoOne/common/gamedata/repository/mall_config"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func MallBuyPackage(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.MallBuyPackageReq{}
	rsp := &g1_protocol.MallBuyPackageRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	defer c.SendMsgBack(rsp)
	rsp.Ret.Code = myRole.MallCheckBuyCondition(req.ConfId)
	if rsp.Ret.Code != 0 {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	conf := mall_config.GetById(req.ConfId)
	if conf == nil {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_CONF
		return rsp.Ret.Code
	}

	_, rsp.Ret.Code = myRole.ItemCheckReduce(conf.CostItemID, int64(conf.CostItemCnt))
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		return rsp.Ret.Code
	}

	//如果是充值购买的礼包就走充值
	if int32(g1_protocol.EItemID_ACECOIN) == conf.CostItemID {
		//ret = RechargeAdd(conf.Rmb, myRole)
	} else {
		rsp.Ret.Code = myRole.ItemExchange(conf.CostItemID, int64(conf.CostItemCnt), conf.PackageID,
			1, &role.Reason{g1_protocol.Reason_REASON_MALL_PACKAGE, req.ConfId})
		if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
			return rsp.Ret.Code
		}
	}

	myRole.MallAddBuyCount(req.ConfId)
	myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_MALL_INFO | g1_protocol.ERoleSectionFlag_INVENTORY_INFO | g1_protocol.ERoleSectionFlag_BASIC_INFO)
	return rsp.Ret.Code
}
