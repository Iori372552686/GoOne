package service

import (
	infosvrv1 "github.com/Iori372552686/GoOne/api/gen/game/infosvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// InfoServiceImpl is the IDL-driven ssrpc implementation for infosvr internal RPCs.
type InfoServiceImpl struct{}

var _ infosvrv1.InfoServiceSS = (*InfoServiceImpl)(nil)

func (s *InfoServiceImpl) GetBriefInfo(ctx *ssrpc.Context, req *g1_protocol.InfoGetBriefInfoReq) (*g1_protocol.InfoGetBriefInfoRsp, error) {
	rsp := &g1_protocol.InfoGetBriefInfoRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	res, ret := globals.InfoMgr.GetInfo(&req.UidList)
	if ret != 0 {
		return rsp, gerr.New(g1_protocol.ErrorCode(ret), "get_info", "")
	}
	if res != nil {
		rsp.InfoList = *res
	}
	return rsp, nil
}

func (s *InfoServiceImpl) GetIconDesc(ctx *ssrpc.Context, req *g1_protocol.InfoGetIconDescReq) (*g1_protocol.InfoGetIconDescRsp, error) {
	rsp := &g1_protocol.InfoGetIconDescRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	res, ret := globals.InfoMgr.GetInfo(&req.UidList)
	if ret != 0 {
		return rsp, gerr.New(g1_protocol.ErrorCode(ret), "get_info", "")
	}

	iconList := make([]*g1_protocol.PbIconDesc, 0)
	if res != nil {
		iconList = make([]*g1_protocol.PbIconDesc, 0, len(*res))
		for _, v := range *res {
			iconList = append(iconList, misc.GetIconDescFromRoleBrief(v))
		}
	}
	rsp.IconList = iconList
	return rsp, nil
}

func (s *InfoServiceImpl) SetBriefInfo(ctx *ssrpc.Context, req *g1_protocol.InfoSetBriefInfoReq) (*g1_protocol.InfoSetBriefInfoRsp, error) {
	rsp := &g1_protocol.InfoSetBriefInfoRsp{
		Ret: &g1_protocol.Ret{
			Code: g1_protocol.ErrorCode(globals.InfoMgr.SetInfo(req.GetUid(), req.GetInfo())),
		},
	}
	if req.GetIgnoreRsp() {
		return nil, nil
	}
	return rsp, nil
}
