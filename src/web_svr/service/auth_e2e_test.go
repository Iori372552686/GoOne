package service

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	http_sign2 "github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/golang/protobuf/proto"
)

// TestE2E_AuthWithChain_MsgSecCheck simulates the exact dispatch chain the
// generated websvr code builds for MsgSecCheck:
//
//	DefaultMiddlewares(Auth: HTTPAuthVerifier, Sign: HTTPSignVerifier)
//
// plus the per-request ctx stamping WrapHTTPGin performs via applyDesc
// (ctx.AuthRequired = desc.Auth; ctx.SignRequired = desc.Sign).
//
// Proves the full auth link:
//
//	proto auth:true -> MethodDesc{Auth:true} -> applyDesc ->
//	ctx.AuthRequired -> AuthWith -> HTTPAuthVerifier.Authenticate.
func TestE2E_AuthWithChain_MsgSecCheck(t *testing.T) {
	signIns := http_sign2.BuildHttpSign("sign", "secret", 60, "timestamp", "request_id", "1")

	// Mirror what bindings.go + DefaultMiddlewares produce for the web service.
	mws := ssrpc.DefaultMiddlewares(ssrpc.DefaultMWOptions{
		Sign: NewHTTPSignVerifier(true, signIns),
		Auth: NewHTTPAuthVerifier(true, signIns),
	})
	chain := ssrpc.Chain(mws...)

	terminalReached := false
	terminal := ssrpc.Handler(func(ctx *ssrpc.Context, req proto.Message) (proto.Message, error) {
		terminalReached = true
		return nil, nil
	})
	h := chain(terminal)

	// Replicate WrapHTTPGin's applyDesc stamping (server.go applyDesc).
	buildCtx := func(body []byte, params map[string]string, authReq, signReq bool) *ssrpc.Context {
		ctx := &ssrpc.Context{}
		ctx.SetHTTPRequest(&http.Request{
			URL: &url.URL{RawQuery: http_sign2.MapParam2Uri(params, false)},
		}, body)
		ctx.AuthRequired = authReq
		ctx.SignRequired = signReq
		return ctx
	}

	// Case 1: MsgSecCheck (Auth+Sign) with valid signature -> terminal reached.
	body := []byte(`{"account_id":"acc","msg_content":"hi","time":"1"}`)
	params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
	signIns.PushSign(params, body)
	if _, err := h(buildCtx(body, params, true, true), nil); err != nil {
		t.Fatalf("valid signature: chain rejected: %v", err)
	}
	if !terminalReached {
		t.Fatalf("valid signature: terminal handler not reached")
	}

	// Case 2: tampered signature -> SignWith or AuthWith rejects -> terminal NOT reached.
	terminalReached = false
	if _, err := h(buildCtx([]byte(`{"msg_content":"tampered"}`), params, true, true), nil); err == nil {
		t.Fatalf("tampered signature: chain should reject")
	}
	if terminalReached {
		t.Fatalf("tampered signature: terminal should NOT be reached")
	}

	// Case 3: Ping (Auth=false, Sign=false) -> both guards skip -> terminal reached.
	terminalReached = false
	if _, err := h(buildCtx(nil, nil, false, false), nil); err != nil {
		t.Fatalf("non-auth method should pass through, got: %v", err)
	}
	if !terminalReached {
		t.Fatalf("non-auth method: terminal not reached")
	}
}
