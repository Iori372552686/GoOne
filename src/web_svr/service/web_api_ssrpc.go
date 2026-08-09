package service

import (
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/api/gen/web/websvr/v1"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	define "github.com/Iori372552686/GoOne/src/web_svr/common"
	"github.com/Iori372552686/GoOne/src/web_svr/web_service"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// WebApiServiceImpl is the Phase-2 HTTP adapter target implementation for web_svr.
// It is wired via generated Dispatcher registration in controller/router.go.
type WebApiServiceImpl struct {
	websvrv1.UnimplementedWebApiServiceSS
}

var _ websvrv1.WebApiServiceSS = (*WebApiServiceImpl)(nil)

func (s *WebApiServiceImpl) Ping(ctx *ssrpc.Context, req *websvrv1.PingReq) (*websvrv1.PingRsp, error) {
	msg := "pong"
	if req != nil && strings.TrimSpace(req.GetMsg()) != "" {
		msg = "pong: " + strings.TrimSpace(req.GetMsg())
	}
	return &websvrv1.PingRsp{
		Msg:          msg,
		ServerUnixMs: time.Now().UnixMilli(),
	}, nil
}

func (s *WebApiServiceImpl) WatchPing(ctx *ssrpc.Context, req *websvrv1.PingReq, stream *ssrpc.ServerStream[*websvrv1.PingRsp]) error {
	msg := "pong"
	if req != nil && strings.TrimSpace(req.GetMsg()) != "" {
		msg = "pong: " + strings.TrimSpace(req.GetMsg())
	}
	for i := 0; i < 3; i++ {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err := stream.Send(&websvrv1.PingRsp{
			Msg:          msg,
			ServerUnixMs: time.Now().UnixMilli(),
		}); err != nil {
			return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "send ping stream response failed", err)
		}
	}
	return nil
}

func (s *WebApiServiceImpl) MsgSecCheck(ctx *ssrpc.Context, req *websvrv1.MsgSecCheckReq) (*websvrv1.MsgSecCheckRsp, error) {
	legacyReq := &define.MsgSecCheckReq{}
	if req != nil {
		legacyReq.AccountId = req.GetAccountId()
		legacyReq.MsgContent = req.GetMsgContent()
		legacyReq.Time = req.GetTime()
	}
	ret := web_service.MsgSecCheck(legacyReq)
	if ret == nil {
		return nil, ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "msg security check internal error", nil)
	}
	if ret.Code != g1_protocol.ErrorCode_ERR_OK {
		return nil, ssrpc.Wrap(ret.Code, ret.Msg, nil)
	}
	return &websvrv1.MsgSecCheckRsp{Msg: "ok"}, nil
}
