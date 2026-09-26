package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
)

// 认证链路：SignedGetWithHeaders 携带 query(account_id/game_id) 与
// headers(Authorization/X-Account-Id)，从 game_profile.uid 解析真实 uid。
// （测试进程未加载 conf，env_mode 为空 ≠ "dev"，走生产认证分支。）
func TestOnCheckAuthByAccSvrFetchesUIDFromAccountProfile(t *testing.T) {
	oldRestMgr := globals.RestMgr
	oldSignMgr := globals.SignMgr
	defer func() {
		globals.RestMgr = oldRestMgr
		globals.SignMgr = oldSignMgr
	}()

	globals.SignMgr = http_sign.NewSignMgr()
	globals.SignMgr.InitAndRun([]http_sign.Config{
		{IndexName: "default", PrivateKey: "test", SignName: "sign", ExpiredTime: 1800, TimestampName: "timestamp", SignType: "md5"},
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("account_id"); got != "10001" {
			t.Fatalf("unexpected account_id query: %s", got)
		}
		if got := r.Header.Get("X-Account-Id"); got != "10001" {
			t.Fatalf("unexpected X-Account-Id header: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Fatalf("unexpected Authorization header: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{
				"account_id": 10001,
				"game_profile": map[string]any{
					"uid": 200123,
				},
			},
		})
	}))
	defer ts.Close()

	globals.RestMgr = rest_api.NewRestApiMgr()
	globals.RestMgr.Init([]rest_api.Config{
		{ServiceName: "default", Urls: []string{ts.URL + "/api/v1/account/profile?"}, SignName: "default"},
	}, globals.SignMgr)

	ok, uid := OnCheckAuthByAccSvr("10001", "Bearer token-abc", 1, "guest")
	if !ok {
		t.Fatalf("expected auth success")
	}
	if uid != 200123 {
		t.Fatalf("expected uid 200123, got %d", uid)
	}
}

// 非 0 返回码 / profile 缺 uid 时必须认证失败。
func TestOnCheckAuthByAccSvrRejectsBadProfile(t *testing.T) {
	oldRestMgr := globals.RestMgr
	oldSignMgr := globals.SignMgr
	defer func() {
		globals.RestMgr = oldRestMgr
		globals.SignMgr = oldSignMgr
	}()

	globals.SignMgr = http_sign.NewSignMgr()
	globals.SignMgr.InitAndRun([]http_sign.Config{
		{IndexName: "default", PrivateKey: "test", SignName: "sign", ExpiredTime: 1800, TimestampName: "timestamp", SignType: "md5"},
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 1001,
			"msg":  "banned",
			"data": map[string]any{},
		})
	}))
	defer ts.Close()

	globals.RestMgr = rest_api.NewRestApiMgr()
	globals.RestMgr.Init([]rest_api.Config{
		{ServiceName: "default", Urls: []string{ts.URL + "/api/v1/account/profile?"}, SignName: "default"},
	}, globals.SignMgr)

	if ok, uid := OnCheckAuthByAccSvr("10001", "token", 1, "guest"); ok || uid != 0 {
		t.Fatalf("expected auth failure, got ok=%v uid=%d", ok, uid)
	}
}

// 空 account / 未配置账号服（RestMgr 空）时认证失败，不产生可信 uid。
func TestOnCheckAuthByAccSvrRejectsMissingAccountAndMissingRestIns(t *testing.T) {
	if ok, uid := OnCheckAuthByAccSvr("  ", "token", 1, "guest"); ok || uid != 0 {
		t.Fatalf("expected auth failure for empty account, got ok=%v uid=%d", ok, uid)
	}

	oldRestMgr := globals.RestMgr
	defer func() { globals.RestMgr = oldRestMgr }()
	globals.RestMgr = rest_api.NewRestApiMgr() // 未 Init：GetRestIns 返回 nil
	if ok, uid := OnCheckAuthByAccSvr("10001", "token", 1, "guest"); ok || uid != 0 {
		t.Fatalf("expected auth failure without rest instance, got ok=%v uid=%d", ok, uid)
	}
}

// parseProfileUID 对 string/float64/int/int64 的容错解析。
func TestParseProfileUID(t *testing.T) {
	cases := []struct {
		in     map[string]any
		want   int64
		wantOK bool
	}{
		{map[string]any{"uid": "42"}, 42, true},
		{map[string]any{"uid": float64(7)}, 7, true},
		{map[string]any{"uid": int(9)}, 9, true},
		{map[string]any{"uid": int64(11)}, 11, true},
		{nil, 0, false},
		{map[string]any{}, 0, false},
		{map[string]any{"uid": true}, 0, false},
	}
	for _, c := range cases {
		got, ok := parseProfileUID(c.in)
		if got != c.want || ok != c.wantOK {
			t.Fatalf("parseProfileUID(%v) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}
