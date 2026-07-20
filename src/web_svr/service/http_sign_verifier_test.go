package service

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	http_sign2 "github.com/Iori372552686/GoOne/lib/web/http_sign"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func TestHTTPSignVerifierVerify(t *testing.T) {
	signIns := http_sign2.BuildHttpSign("sign", "secret", 60, "timestamp", "request_id", "1")
	body := []byte(`{"account_id":"acc","msg_content":"hello","time":"123"}`)
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	signIns.PushSign(&params, body, http_sign2.Sign_Md5)

	ctx := &ssrpc.Context{}
	ctx.SetHTTPRequest(&http.Request{
		URL: &url.URL{RawQuery: http_sign2.MapParam2Uri(&params, false)},
	}, body)

	verifier := NewHTTPSignVerifier(true, signIns)
	if err := verifier.Verify(ctx, nil); err != nil {
		t.Fatalf("expected valid signature, got err=%v", err)
	}

	badCtx := &ssrpc.Context{}
	badCtx.SetHTTPRequest(&http.Request{
		URL: &url.URL{RawQuery: http_sign2.MapParam2Uri(&params, false)},
	}, []byte(`{"account_id":"acc","msg_content":"tampered","time":"123"}`))

	err := verifier.Verify(badCtx, nil)
	if err == nil {
		t.Fatalf("expected invalid signature error")
	}
	if got := ssrpc.ToErrorCode(err); got != g1_protocol.ErrorCode_ERR_FAIL {
		t.Fatalf("expected ERR_FAIL, got %v err=%v", got, err)
	}
}

func TestHTTPSignVerifierVerifyDisabled(t *testing.T) {
	verifier := NewHTTPSignVerifier(false, nil)
	if err := verifier.Verify(&ssrpc.Context{}, nil); err != nil {
		t.Fatalf("expected disabled verifier to skip, got err=%v", err)
	}
}
