package cmd_handler

import (
	`bian/src/bian_newFrame/lib/cmd_handler`
	g1_protocol `bian/src/bian_newFrame/protobuf/protocol`
	`bian/src/bian_newFrame/src/mysqlsvr/globals`
	`bian/src/common/logger`
)

type UpdateRoleInfo struct{}

func (h *UpdateRoleInfo) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.MysqlInnerUpdateRoleInfoReq{}
	rsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0
	for {
		instance := uint32(g1_protocol.EMysqlType_MYSQL_TYPE_ROLE_INFO)
		if checkRoleExist(c) {
			c.Infof("role exist")
			_, err = globals.MysqlMgr.Execute(instance, "UPDATE role_info SET name = ? WHERE uid = ?",
				req.Name, c.Uid())
			if err != nil {
				logger.Errorf("failed to update role info | %v", err)
				ret = -1
				break
			}
		} else {
			c.Infof("role not exist")
			_, err = globals.MysqlMgr.Execute(instance, "INSERT INTO role_info VALUES (?, ?)", c.Uid(), req.Name)
			if err != nil {
				logger.Errorf("failed to insert role info | %v", err)
				ret = -1
				break
			}
		}
		break
	}

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(rsp)
	return ret
}

type SearchRole struct{}

func (h *SearchRole) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	req := &g1_protocol.MysqlInnerSearchRoleReq{}
	rsp := &g1_protocol.MysqlInnerSearchRoleRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}

	ret := 0
	for {
		instance := uint32(g1_protocol.EMysqlType_MYSQL_TYPE_ROLE_INFO)
		rows, e := globals.MysqlMgr.Query(instance, "SELECT uid FROM role_info WHERE name = (?)",
			req.SearchString)
		if e != nil {
			logger.Errorf("failed to select role info: %v", e)
			ret = -1
			break
		}

		var uid uint64
		for rows.Next() {
			err := rows.Scan(&uid)
			if err != nil {
				logger.Errorf("scan error: %v", err)
			}
		}

		rsp.Uid = uid
		break
	}

	rsp.Ret = &g1_protocol.Ret{Ret: int32(ret)}
	c.SendMsgBack(rsp)
	return ret
}


func checkRoleExist(c cmd_handler.IContext) bool {
	instance := uint32(g1_protocol.EMysqlType_MYSQL_TYPE_ROLE_INFO)
	res, err := globals.MysqlMgr.Query(instance, "SELECT uid FROM role_info where uid = (?)", c.Uid())
	if err != nil {
		logger.Errorf("failed to check role exist | %v", err)
	}
	return res != nil && res.Next()
}

func checkGiftCodeExist(giftCode string) bool {
	instance := uint32(g1_protocol.EMysqlType_MYSQL_TYPE_ROLE_INFO)
	res, err := globals.MysqlMgr.Query(instance, "SELECT id FROM gift_info WHERE gift_code = (?)", giftCode)
	if err != nil {
		logger.Errorf("failed to check role exist | %v", err)
	}
	return res != nil && res.Next()
}
