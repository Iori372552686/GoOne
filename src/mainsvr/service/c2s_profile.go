package service

import (
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// ===== 资料：改名 / 头像装扮 =====

func (s *MainC2SServiceImpl) ChangeName(ctx *ssrpc.Context, req *g1_protocol.ChangeNameReq) (*g1_protocol.ChangeNameRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.ChangeNameRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	// 检查是否满足条件
	free := myRole.PbRole.BasicInfo.GetFreeCnt()
	_, hasCoin := myRole.ItemCheckReduce(int32(g1_protocol.EItemID_GOLD), 100)
	if hasCoin != 0 && free <= 0 {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH, "biz_error", "")
	}

	// 检查敏感字
	hasSensitiveWord, _ := sensitive_words.ChangeSensitiveWords(req.GetName())
	if hasSensitiveWord {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INVALID_NAME, "biz_error", "")
	}

	// 同步数据
	myRole.PbRole.BasicInfo.Name = req.GetName()
	myRole.TouchBasicInfo("change_name")
	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}

func (s *MainC2SServiceImpl) ChangeIcon(ctx *ssrpc.Context, req *g1_protocol.ChangeIconReq) (*g1_protocol.ChangeIconRsp, error) {
	myRole := s.roles.GetOrLoadRole(ctx.Uid(), ctx)
	if myRole == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_ARGV, "role not found")
	}

	rsp := &g1_protocol.ChangeIconRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}

	if req.GetIconId() > 0 {
		code := myRole.IconChange(req.GetIconId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "icon_change", "")
		}
	}
	if req.GetFrameId() > 0 {
		code := myRole.FrameChange(req.GetFrameId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "frame_change", "")
		}
	}
	if req.GetImageId() > 0 {
		code := myRole.ImageChange(req.GetImageId())
		if code != g1_protocol.ErrorCode_ERR_OK {
			return rsp, gerr.New(code, "image_change", "")
		}
	}

	_ = myRole.FlushPending(ctx, false)
	return rsp, nil
}
