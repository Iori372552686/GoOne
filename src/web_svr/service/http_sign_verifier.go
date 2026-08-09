package service

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	http_sign2 "github.com/Iori372552686/GoOne/lib/web/http_sign"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// HTTPSignVerifier 将 http_sign.HttpSign 适配为 ssrpc.SignVerifier，
// 以便挂载到请求中间件。
type HTTPSignVerifier struct {
	enabled bool
	signIns *http_sign2.HttpSign
}

// NewHTTPSignVerifier 返回一个校验器；当 enabled 为 false 时它即为空操作。
func NewHTTPSignVerifier(enabled bool, signIns *http_sign2.HttpSign) *HTTPSignVerifier {
	return &HTTPSignVerifier{enabled: enabled, signIns: signIns}
}

var _ ssrpc.SignVerifier = (*HTTPSignVerifier)(nil)

// Verify 校验入站 HTTP 请求的签名。校验关闭或通过时返回 nil，
// 否则返回一个 ssrpc 包装错误，错误信息中携带失败的 http_sign ErrorCode。
func (v *HTTPSignVerifier) Verify(ctx *ssrpc.Context, _ proto.Message) error {
	if v == nil || !v.enabled {
		return nil
	}
	if v.signIns == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "http sign verifier not configured", nil)
	}
	if ctx == nil || ctx.HTTPRequest == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "missing http request for sign verification", nil)
	}

	if code, err := v.signIns.CheckSign(
		http_sign2.UriParam2Map(ctx.HTTPRequest.URL.RawQuery), ctx.HTTPBody, "",
	); err == nil {
		return nil
	} else {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_FAIL,
			fmt.Sprintf("invalid signature: code=%s err=%v", code, err), err)
	}
}
