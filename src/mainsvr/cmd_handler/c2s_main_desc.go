package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/service/sensitive_words"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

func ChangeName(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.ChangeNameReq{}
	rsp := &g1_protocol.ChangeNameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}
	defer c.SendMsgBack(rsp)

	// 检查是否满足条件
	free := myRole.PbRole.BasicInfo.GetFreeCnt()
	_, hasCoin := myRole.ItemCheckReduce(int32(g1_protocol.EItemID_GOLD), 100)
	if hasCoin != 0 && free <= 0 {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH
		return rsp.Ret.Code
	}

	// 检查敏感字
	hasSensitiveWord, _ := sensitive_words.ChangeSensitiveWords(req.Name)
	if hasSensitiveWord {
		rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INVALID_NAME
		return rsp.Ret.Code
	}

	// 检查重复
	/*	dupCheckRsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{}
		err = c.CallMsgBySvrType(misc.ServerType_MysqlSvr, g1_protocol.CMD_MYSQL_INNER_UPDATE_ROLE_INFO_REQ,
			&g1_protocol.MysqlInnerUpdateRoleInfoReq{Name: req.Name}, dupCheckRsp)
		if err != nil {
			myRole.Errorf("update mysql role error: %v", err)
			rsp.Ret.Code = g1_protocol.ErrorCode_ERR_INTERNAL
			return rsp.Ret.Code
		}

		if dupCheckRsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
			rsp.Ret.Code = g1_protocol.ErrorCode_ERR_DUPLICATE_NAME
			return rsp.Ret.Code
		}*/

	// 同步数据
	myRole.PbRole.BasicInfo.Name = req.Name
	myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_BASIC_INFO)
	return rsp.Ret.Code
}

func ChangeIcon(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode {
	req := &g1_protocol.ChangeIconReq{}
	rsp := &g1_protocol.ChangeIconRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	defer c.SendMsgBack(rsp)

	if req.IconId > 0 {
		rsp.Ret.Code = myRole.IconChange(req.IconId)
		if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
			return rsp.Ret.Code
		}
	}
	if req.FrameId > 0 {
		rsp.Ret.Code = myRole.FrameChange(req.FrameId)
		if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
			return rsp.Ret.Code
		}
	}
	if req.ImageId > 0 {
		rsp.Ret.Code = myRole.ImageChange(req.ImageId)
		if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
			return rsp.Ret.Code
		}
	}

	myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_ICON_INFO)
	return rsp.Ret.Code
}
