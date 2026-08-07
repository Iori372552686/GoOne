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

func TestHTTPAuthVerifier_AuthenticateDisabled(t *testing.T) {
	v := NewHTTPAuthVerifier(false, nil)
	if err := v.Authenticate(&ssrpc.Context{}, nil); err != nil {
		t.Fatalf("expected disabled authenticator to skip, got err=%v", err)
	}
}

func TestHTTPAuthVerifier_NotConfigured(t *testing.T) {
	v := NewHTTPAuthVerifier(true, nil)
	err := v.Authenticate(&ssrpc.Context{}, nil)
	if err == nil {
		t.Fatalf("expected ERR_INTERNAL when signIns is nil")
	}
	if got := ssrpc.ToErrorCode(err); got != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected ERR_INTERNAL, got %v err=%v", got, err)
	}
}

func TestHTTPAuthVerifier_MissingHTTPRequest(t *testing.T) {
	signIns := http_sign2.BuildHttpSign("sign", "secret", 60, "timestamp", "request_id", "1")
	v := NewHTTPAuthVerifier(true, signIns)
	err := v.Authenticate(&ssrpc.Context{}, nil) // no HTTPRequest set
	if err == nil {
		t.Fatalf("expected ERR_INTERNAL when HTTPRequest is missing")
	}
	if got := ssrpc.ToErrorCode(err); got != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected ERR_INTERNAL, got %v err=%v", got, err)
	}
}

func TestHTTPAuthVerifier_ValidSignature(t *testing.T) {
	signIns := http_sign2.BuildHttpSign("sign", "secret", 60, "timestamp", "request_id", "1")
	body := []byte(`{"account_id":"acc","msg_content":"hello","time":"123"}`)
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	signIns.PushSign(params, body)

	ctx := &ssrpc.Context{}
	ctx.SetHTTPRequest(&http.Request{
		URL: &url.URL{RawQuery: http_sign2.MapParam2Uri(params, false)},
	}, body)

	v := NewHTTPAuthVerifier(true, signIns)
	if err := v.Authenticate(ctx, nil); err != nil {
		t.Fatalf("expected valid signature to pass, got err=%v", err)
	}
}

func TestHTTPAuthVerifier_InvalidSignature(t *testing.T) {
	signIns := http_sign2.BuildHttpSign("sign", "secret", 60, "timestamp", "request_id", "1")
	body := []byte(`{"account_id":"acc","msg_content":"hello","time":"123"}`)
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	signIns.PushSign(params, body)

	// tampered body -> signature no longer matches.
	ctx := &ssrpc.Context{}
	ctx.SetHTTPRequest(&http.Request{
		URL: &url.URL{RawQuery: http_sign2.MapParam2Uri(params, false)},
	}, []byte(`{"account_id":"acc","msg_content":"tampered","time":"123"}`))

	v := NewHTTPAuthVerifier(true, signIns)
	err := v.Authenticate(ctx, nil)
	if err == nil {
		t.Fatalf("expected invalid signature error")
	}
	if got := ssrpc.ToErrorCode(err); got != g1_protocol.ErrorCode_ERR_FAIL {
		t.Fatalf("expected ERR_FAIL, got %v err=%v", got, err)
	}
}
