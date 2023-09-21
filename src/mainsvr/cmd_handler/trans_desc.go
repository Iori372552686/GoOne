package cmd_handler

import (
	"GoOne/common/misc"
	"GoOne/lib/api/cmd_handler"
	"GoOne/lib/service/sensitive_words"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/mainsvr/role"
)

type ChangeName struct{}

func (h *ChangeName) ProcessCmd(c cmd_handler.IContext, data []byte, myRole *role.Role) int {
	req := &g1_protocol.ChangeNameReq{}
	rsp := &g1_protocol.ChangeNameRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}
	ret := 0

	for {

		// 检查是否满足条件
		free := myRole.PbRole.DescInfo.GetFreeCnt()
		_, hasCard := myRole.ItemCheckReduce(int32(g1_protocol.EItemID_CHANGE_NAME_CARD), 1)
		_, hasDiamond := myRole.ItemCheckReduce(int32(g1_protocol.EItemID_DIAMOND), 100)
		if hasCard != 0 && hasDiamond != 0 && free <= 0 {
			break
		}

		// 检查敏感字
		hasSensitiveWord, _ := sensitive_words.ChangeSensitiveWords(req.Name)
		if hasSensitiveWord {
			ret = int(g1_protocol.ErrorCode_ERR_INVALID_NAME)
			break
		}

		// 检查重复
		dupCheckReq := &g1_protocol.MysqlInnerUpdateRoleInfoReq{}
		dupCheckReq.Name = req.Name
		dupCheckRsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{}
		err = c.CallMsgBySvrType(misc.ServerType_MysqlSvr, uint32(g1_protocol.CMD_MYSQL_INNER_UPDATE_ROLE_INFO_REQ),
			dupCheckReq, dupCheckRsp)
		if err != nil {
			myRole.Errorf("update mysql role error: %v", err)
			ret = -1
			break
		}
		if dupCheckRsp.Ret.Ret != 0 {
			ret = int(g1_protocol.ErrorCode_ERR_DUPLICATE_NAME)
			break
		}

		myRole.PbRole.DescInfo.Name = req.Name

		// 同步数据
		_ = myRole.SyncDataToClient(g1_protocol.ERoleSectionFlag_DESC_INFO | g1_protocol.ERoleSectionFlag_INVENTORY_INFO)
		break
	}

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(rsp)

	return ret
}
