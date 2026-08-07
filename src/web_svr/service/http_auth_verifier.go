package service

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	http_sign2 "github.com/Iori372552686/GoOne/lib/web/http_sign"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

// HTTPAuthVerifier adapts http_sign.HttpSign into an ssrpc.Authenticator so
// that the per-method auth:true flag can be enforced via the AuthWith
// middleware.
//
// It reuses the same HTTP signature check as HTTPSignVerifier. The two can be
// mounted together (SignWith + AuthWith); for an auth+sign method the signature
// is verified twice. This mirrors the existing web_svr auth model, where HTTP
// signature is the only available identity signal (there is no uid/token on the
// HTTP transport: httpIContext.Uid() always returns 0).
type HTTPAuthVerifier struct {
	enabled bool
	signIns *http_sign2.HttpSign
}

// NewHTTPAuthVerifier returns an authenticator that is a no-op when enabled is
// false.
func NewHTTPAuthVerifier(enabled bool, signIns *http_sign2.HttpSign) *HTTPAuthVerifier {
	return &HTTPAuthVerifier{enabled: enabled, signIns: signIns}
}

var _ ssrpc.Authenticator = (*HTTPAuthVerifier)(nil)

// Authenticate enforces the inbound HTTP signature when AuthRequired is set.
// Returns nil when auth is disabled or the signature is valid, otherwise an
// ssrpc-wrapped error carrying the failed http_sign ErrorCode.
func (v *HTTPAuthVerifier) Authenticate(ctx *ssrpc.Context, _ proto.Message) error {
	if v == nil || !v.enabled {
		return nil
	}
	if v.signIns == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "http auth verifier not configured", nil)
	}
	if ctx == nil || ctx.HTTPRequest == nil {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "missing http request for auth verification", nil)
	}

	if code, err := v.signIns.CheckSign(
		http_sign2.UriParam2Map(ctx.HTTPRequest.URL.RawQuery), ctx.HTTPBody, "",
	); err == nil {
		return nil
	} else {
		return ssrpc.Wrap(g1_protocol.ErrorCode_ERR_FAIL,
			fmt.Sprintf("auth invalid signature: code=%s err=%v", code, err), err)
	}
}
