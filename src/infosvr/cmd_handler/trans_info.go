package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	g1_protocol "github.com/gdsgog/poker_protocol/protocol"
)

func GetBriefInfo(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.InfoGetBriefInfoReq{}
	rsp := &g1_protocol.InfoGetBriefInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := 0
	for {
		res, r := globals.InfoMgr.GetInfo(&req.UidList)
		if r != 0 {
			ret = r
			break
		}

		if res != nil {
			rsp.InfoList = *res
		}

		break
	}
	rsp.Ret = &g1_protocol.Ret{Code: g1_protocol.ErrorCode(ret)}

	c.SendMsgBack(rsp)
	return g1_protocol.ErrorCode(ret)
}

func GetIconDesc(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.InfoGetIconDescReq{}
	rsp := &g1_protocol.InfoGetIconDescRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := g1_protocol.ErrorCode_ERR_OK
	for {
		res, r := globals.InfoMgr.GetInfo(&req.UidList)
		if r != 0 {
			ret = g1_protocol.ErrorCode(r)
			break
		}

		iconList := make([]*g1_protocol.PbIconDesc, 0, len(*res))
		for _, v := range *res {
			icon := misc.GetIconDescFromRoleBrief(v)
			iconList = append(iconList, icon)
		}
		rsp.IconList = iconList

		break
	}
	rsp.Ret = &g1_protocol.Ret{Code: ret}

	c.SendMsgBack(rsp)
	return ret
}

func SetBriefInfo(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	req := &g1_protocol.InfoSetBriefInfoReq{}
	rsp := &g1_protocol.InfoSetBriefInfoRsp{}
	err := c.ParseMsg(data, req)
	if err != nil {
		return g1_protocol.ErrorCode_ERR_MARSHAL
	}

	ret := 0
	for {
		ret = globals.InfoMgr.SetInfo(req.Uid, req.Info)
		if ret != 0 {
			break
		}

		break
	}
	rsp.Ret = &g1_protocol.Ret{Code: g1_protocol.ErrorCode(ret)}
	if !req.IgnoreRsp {
		c.SendMsgBack(rsp)
	}
	return g1_protocol.ErrorCode(ret)
}
