package service

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	http_sign2 "github.com/Iori372552686/GoOne/lib/web/http_sign"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type HTTPSignVerifier struct {
	enabled bool
	signIns *http_sign2.HttpSign
}

func NewHTTPSignVerifier(enabled bool, signIns *http_sign2.HttpSign) *HTTPSignVerifier {
	return &HTTPSignVerifier{
		enabled: enabled,
		signIns: signIns,
	}
}

var _ ssrpc.SignVerifier = (*HTTPSignVerifier)(nil)

func (v *HTTPSignVerifier) Verify(ctx *ssrpc.Context, req proto.Message) error {
	_ = req

	if v == nil || !v.enabled {
		return nil
	}
	if v.signIns == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "http sign verifier not configured", nil)
	}
	if ctx == nil || ctx.HTTPRequest == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "missing http request for sign verification", nil)
	}

	ok, err, _ := v.signIns.CheckSign(http_sign2.UriParam2Map(ctx.HTTPRequest.URL.RawQuery), ctx.HTTPBody, "")
	if ok {
		return nil
	}

	return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_FAIL, fmt.Sprintf("Invalid signature ! err | %v", err), err)
}
