package rest_api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	apiclient "github.com/Iori372552686/GoOne/lib/api/http_client"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
)

// testSigner 构造一个固定的 HttpSign 实例，供服务端校验与客户端签名共用。
// expiredTime 设大，避免单测因时钟漂移误判。
func testSigner() *http_sign.HttpSign {
	return http_sign.BuildHttpSign("sign", "mysecret", 3600, "timestamp", "", "1")
}

// signMgrWith 构造一个含 default 实例的 SignMgr，便于走 NewRestApi 的按名解析路径。
func signMgrWith(name string, s *http_sign.HttpSign) *http_sign.SignMgr {
	m := http_sign.NewSignMgr()
	m.SetSignIns(name, s)
	return m
}

// recordingServer 起一个本地服务，记录命中次数与请求元数据，并按 verify 决定是否验签。
// 返回的 addr 已含末尾问号，匹配线上 URL 配置习惯（Map2uri 直接拼接 query）。
func recordingServer(t *testing.T, verify func(r *http.Request) (int, string)) (string, *hitCounter) {
	t.Helper()
	hc := &hitCounter{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hc.add()
		hc.setLast(r)
		code, body := verify(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	// 末尾问号是配置约定，rest_api 直接拼 query，不自行补 ?
	return srv.URL + "?", hc
}

type hitCounter struct {
	n     atomic.Int64
	last  http.Request
	mu    sync.RWMutex
	lset  bool
}

func (h *hitCounter) add()                  { h.n.Add(1) }
func (h *hitCounter) count() int64          { return h.n.Load() }
func (h *hitCounter) setLast(r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = *r
	h.lset = true
}
func (h *hitCounter) lastReq() (http.Request, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.last, h.lset
}

// extractQuery 把 r.URL.RawQuery 解析成 map（值含 = 也安全，复用 http_sign.UriParam2Map）。
func extractQuery(r *http.Request) map[string]string {
	return http_sign.UriParam2Map(r.URL.RawQuery)
}

// ---------- 正常链路：真实签名往返 ----------

// TestSignedPost_RoundTrip 客户端签名 → 服务端用同一 HttpSign 验签 → 回固定 JSON。
// 守护签名链路端到端正确，并对齐 http_sign.CheckSign 的 ErrorCode 约定。
func TestSignedPost_RoundTrip(t *testing.T) {
	signer := testSigner()
	addr, hc := recordingServer(t, func(r *http.Request) (int, string) {
		if r.Method != "POST" {
			return 500, `{"err":"method"}`
		}
		body, _ := io.ReadAll(r.Body)
		params := extractQuery(r)
		if code, err := signer.CheckSign(params, body, ""); code != http_sign.SignOK || err != nil {
			t.Errorf("服务端验签失败 code=%s err=%v", code, err)
			return 401, `{"status":false}`
		}
		if r.Header.Get("Authorization") != "Bearer-TOKEN" {
			t.Errorf("header 未透传: %q", r.Header.Get("Authorization"))
		}
		return 200, `{"status":true,"data":{"id":7700}}`
	})

	api := NewRestApi(Config{ServiceName: "auth", Urls: []string{addr}, SignName: "default"},
		signMgrWith("default", signer))
	if api == nil {
		t.Fatal("NewRestApi 返回 nil")
	}

	body := []byte(`{"account_id":"alice"}`)
	header := map[string]string{"Authorization": "Bearer-TOKEN"}
	resp, err := api.SignedPost(context.Background(), 0, nil, body, header)
	if err != nil {
		t.Fatalf("SignedPost 出错: %v", err)
	}
	if string(resp) != `{"status":true,"data":{"id":7700}}` {
		t.Fatalf("响应体不符: %s", resp)
	}
	if hc.count() != 1 {
		t.Fatalf("命中次数=%d, 期望 1", hc.count())
	}
}

// TestSignedGet_RoundTrip GET 链路：签名进 query string，body 为空参与签名。
func TestSignedGet_RoundTrip(t *testing.T) {
	signer := testSigner()
	addr, hc := recordingServer(t, func(r *http.Request) (int, string) {
		if r.Method != "GET" {
			return 500, `{"err":"method"}`
		}
		params := extractQuery(r)
		if code, err := signer.CheckSign(params, nil, ""); code != http_sign.SignOK || err != nil {
			t.Errorf("GET 验签失败 code=%s err=%v", code, err)
			return 401, `{"status":false}`
		}
		return 200, `{"status":true}`
	})

	api := NewRestApi(Config{ServiceName: "auth", Urls: []string{addr}, SignName: "default"},
		signMgrWith("default", signer))
	if _, err := api.SignedGet(context.Background(), 0, map[string]string{"user": "alice"}); err != nil {
		t.Fatalf("SignedGet 出错: %v", err)
	}
	if hc.count() != 1 {
		t.Fatalf("命中次数=%d, 期望 1", hc.count())
	}
}

// TestPost_Plain / TestGet_Plain 普通无签名链路。
func TestPost_Plain(t *testing.T) {
	addr, hc := recordingServer(t, func(r *http.Request) (int, string) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"hi":1}` {
			t.Errorf("POST body 透传不符: %q", body)
		}
		return 200, `{"ok":true}`
	})
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil)
	resp, err := api.Post(context.Background(), 0, []byte(`{"hi":1}`), nil)
	if err != nil {
		t.Fatalf("Post 出错: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Fatalf("响应体不符: %s", resp)
	}
	if hc.count() != 1 {
		t.Fatalf("命中次数=%d", hc.count())
	}
}

func TestGet_Plain(t *testing.T) {
	addr, _ := recordingServer(t, func(r *http.Request) (int, string) {
		if v := r.URL.Query().Get("user"); v != "alice" {
			t.Errorf("query user=%q", v)
		}
		return 200, `{"ok":true}`
	})
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil)
	resp, err := api.Get(context.Background(), 0, map[string]string{"user": "alice"})
	if err != nil {
		t.Fatalf("Get 出错: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Fatalf("响应体不符: %s", resp)
	}
}

// ---------- map 非变异守护（C2 修复回归） ----------

// TestSignedPost_QueryNotMutated 签名后调用方原始 query map 不应被就地改写。
// http_sign.PushSign 会写入 timestamp/sign，rest_api 必须 clone 而非透传。
func TestSignedPost_QueryNotMutated(t *testing.T) {
	signer := testSigner()
	addr, _ := recordingServer(t, func(r *http.Request) (int, string) { return 200, `{}` })

	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}, SignName: "default"},
		signMgrWith("default", signer))
	q := map[string]string{"user": "alice"}
	before := map[string]string{"user": "alice"}
	if _, err := api.SignedPost(context.Background(), 0, q, []byte("{}"), nil); err != nil {
		t.Fatalf("SignedPost 出错: %v", err)
	}
	if len(q) != len(before) {
		t.Fatalf("query 被改写: len=%d 期望 %d (map 被就地注入了 sign 字段)", len(q), len(before))
	}
	for k, v := range before {
		if q[k] != v {
			t.Fatalf("query 被改写: key=%s got=%s want=%s", k, q[k], v)
		}
	}
}

// ---------- 错误路径 ----------

// TestNilReceiver 所有方法对 nil receiver 返回错误而非 panic。
func TestNilReceiver(t *testing.T) {
	var api *RestApi
	ctx := context.Background()
	if _, err := api.Get(ctx, 0, nil); err == nil {
		t.Fatal("nil receiver Get 应报错")
	}
	if _, err := api.SignedGet(ctx, 0, nil); err == nil {
		t.Fatal("nil receiver SignedGet 应报错")
	}
	if _, err := api.Post(ctx, 0, nil, nil); err == nil {
		t.Fatal("nil receiver Post 应报错")
	}
	if _, err := api.SignedPost(ctx, 0, nil, nil, nil); err == nil {
		t.Fatal("nil receiver SignedPost 应报错")
	}
}

// TestSignedWithoutSignInstance 需签名但未装配 HttpSign 时返回 errNoSign。
func TestSignedWithoutSignInstance(t *testing.T) {
	addr, _ := recordingServer(t, func(*http.Request) (int, string) { return 200, `{}` })
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil) // 无签名
	if api.sign != nil {
		t.Fatal("未配置 sign 不应装配 HttpSign")
	}
	_, err := api.SignedPost(context.Background(), 0, nil, []byte("{}"), nil)
	if !errors.Is(err, errNoSign) {
		t.Fatalf("期望 errNoSign, got %v", err)
	}
	_, err = api.SignedGet(context.Background(), 0, nil)
	if !errors.Is(err, errNoSign) {
		t.Fatalf("期望 errNoSign, got %v", err)
	}
}

// TestServer5xx 非导出：底层 DoRequest 对非 2xx 返回带状态码的 error，且仍走 err 分支。
func TestServer5xx(t *testing.T) {
	addr, _ := recordingServer(t, func(*http.Request) (int, string) { return 500, `boom` })
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil)
	resp, err := api.Get(context.Background(), 0, nil)
	if err == nil {
		t.Fatal("5xx 应返回 error")
	}
	if resp != nil {
		t.Fatalf("出错应返回 nil body（修复 B1）, got %s", resp)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("错误应含状态码: %v", err)
	}
}

// TestContextTimeout 调用方 ctx 提前到期应触发超时错误，且不阻塞 8s。
func TestContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // 故意慢于 ctx
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	api := NewRestApi(Config{ServiceName: "s", Urls: []string{srv.URL + "?"}}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := api.Get(ctx, 0, nil); err == nil {
		t.Fatal("ctx 超时应返回 error")
	}
}

// TestInvalidConfigSkipped NewRestApi 对缺 ServiceName/Urls 返回 nil（守护 Init 上报分支）。
func TestInvalidConfigSkipped(t *testing.T) {
	if NewRestApi(Config{}, nil) != nil {
		t.Fatal("空 Config 应返回 nil")
	}
	if NewRestApi(Config{ServiceName: "x"}, nil) != nil {
		t.Fatal("缺 Urls 应返回 nil")
	}
	if NewRestApi(Config{Urls: []string{"http://x?"}}, nil) != nil {
		t.Fatal("缺 ServiceName 应返回 nil")
	}
}

// ---------- URL 分片 ----------

// TestUrlPool_ShardByUin uin!=0 时取模分片，确定性。
func TestUrlPool_ShardByUin(t *testing.T) {
	p := newUrlPool([]string{"a", "b", "c"})
	cases := map[int64]string{
		1: "b", 2: "c", 3: "a", 6: "a", 7: "b",
	}
	for uin, want := range cases {
		if got := p.pick(uin); got != want {
			t.Errorf("pick(%d)=%q want %q", uin, got, want)
		}
	}
}

// TestUrlPool_SingleSingle 单元素直接返回。
func TestUrlPool_SingleSingle(t *testing.T) {
	p := newUrlPool([]string{"only"})
	if got := p.pick(0); got != "only" {
		t.Fatalf("单元素期望 only, got %q", got)
	}
}

// TestUrlPool_RoundRobin uin==0 时 atomic 轮询，N 次后每个后端都被命中。
func TestUrlPool_RoundRobin(t *testing.T) {
	urls := []string{"a", "b", "c"}
	p := newUrlPool(urls)
	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		seen[p.pick(0)]++
	}
	if len(seen) != 3 {
		t.Fatalf("轮询应覆盖全部后端, got %v", seen)
	}
	for _, u := range urls {
		if seen[u] == 0 {
			t.Fatalf("后端 %s 未被轮到", u)
		}
	}
}

// TestMultiUrl_HitDistribution 端到端验证多后端分片：固定 uin 始终命中同一后端。
func TestMultiUrl_HitDistribution(t *testing.T) {
	var hits struct {
		sync.Mutex
		a, b int
	}
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Lock()
		hits.a++
		hits.Unlock()
		w.WriteHeader(200)
	}))
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Lock()
		hits.b++
		hits.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srvA.Close)
	t.Cleanup(srvB.Close)

	// 注意：urlPool.pick 按 urls 的顺序取模；保证 srvA 对应 idx 0、srvB 对应 idx 1。
	api := NewRestApi(Config{
		ServiceName: "s",
		Urls:        []string{srvA.URL + "?", srvB.URL + "?"},
	}, nil)
	for i := 0; i < 5; i++ {
		if _, err := api.Get(context.Background(), 2, nil); err != nil { // 2%2==0 → srvA
			t.Fatalf("Get 出错: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		if _, err := api.Get(context.Background(), 1, nil); err != nil { // 1%2==1 → srvB
			t.Fatalf("Get 出错: %v", err)
		}
	}
	hits.Lock()
	defer hits.Unlock()
	if hits.a != 5 || hits.b != 5 {
		t.Fatalf("分片命中 a=%d b=%d, 期望各 5", hits.a, hits.b)
	}
}

// ---------- Manager ----------

// TestManager_Basic 覆盖 Init/Set/Get(默认与命名)/Count/nil 安全。
func TestManager_Basic(t *testing.T) {
	m := NewRestApiMgr()
	if m.Count() != 0 {
		t.Fatalf("空 mgr Count=%d", m.Count())
	}

	m.Init([]Config{
		{ServiceName: "default", Urls: []string{"http://a?"}},
		{ServiceName: "pay", Urls: []string{"http://b?"}},
		{}, // 非法，应被跳过且不 panic
	}, nil)

	if m.Count() != 2 {
		t.Fatalf("Init 后 Count=%d, 期望 2", m.Count())
	}
	if m.GetRestIns() == nil {
		t.Fatal("GetRestIns() 默认应返回 default 实例")
	}
	if m.GetRestIns("pay") == nil {
		t.Fatal("GetRestIns(\"pay\") 不应为 nil")
	}
	if m.GetRestIns("nope") != nil {
		t.Fatal("未知 key 应返回 nil")
	}
}

// TestManager_NilSafe nil mgr 的 Count 不 panic。
func TestManager_NilSafe(t *testing.T) {
	var m *RestApiMgr
	if m.Count() != 0 {
		t.Fatal("nil mgr Count 应为 0")
	}
}

// ---------- Options ----------

// TestWithTimeout 兜底超时在 ctx 无 deadline 时生效，且不影响 ctx 自带 deadline。
func TestWithTimeout(t *testing.T) {
	addr, _ := recordingServer(t, func(*http.Request) (int, string) { return 200, `{}` })
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil, WithTimeout(30*time.Second))
	if api.timeout != 30*time.Second {
		t.Fatalf("WithTimeout 未生效: %v", api.timeout)
	}
	// 覆盖：<=0 不生效
	api2 := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}}, nil, WithTimeout(0))
	if api2.timeout != 0 {
		t.Fatalf("WithTimeout(0) 不应设置: %v", api2.timeout)
	}
}

// TestApplyTimeout 覆盖 applyTimeout 的三条分支。
func TestApplyTimeout(t *testing.T) {
	// d<=0：cancel 为 nil
	if _, c := applyTimeout(context.Background(), 0); c != nil {
		t.Fatal("d<=0 时 cancel 应为 nil")
	}
	// ctx 已有 deadline：cancel 为 nil（不覆盖）
	ctx, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer cancelDeadline()
	if _, c := applyTimeout(ctx, time.Second); c != nil {
		t.Fatal("ctx 已有 deadline 时 cancel 应为 nil")
	}
	// 正常叠加：cancel 非 nil
	_, c := applyTimeout(context.Background(), time.Second)
	if c == nil {
		t.Fatal("正常叠加应返回非 nil cancel")
	}
	c()
}

// TestConfigTimeoutFromSeconds Config.Timeout（秒）正确换算成 Duration。
func TestConfigTimeoutFromSeconds(t *testing.T) {
	addr, _ := recordingServer(t, func(*http.Request) (int, string) { return 200, `{}` })
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}, Timeout: 5}, nil)
	if api.timeout != 5*time.Second {
		t.Fatalf("Config.Timeout=5 未换算: %v", api.timeout)
	}
}

// TestConfigTimeout_AppliedInRequest 端到端验证 Config.Timeout 在 ctx 无 deadline 时
// 被 do() 内 applyTimeout 包裹（命中 cancel != nil 分支）。成功路径表明超时未提前触发。
func TestConfigTimeout_AppliedInRequest(t *testing.T) {
	addr, _ := recordingServer(t, func(*http.Request) (int, string) { return 200, `{}` })
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}, Timeout: 5}, nil)
	if _, err := api.Get(context.Background(), 0, nil); err != nil {
		t.Fatalf("Config.Timeout 兜底路径出错: %v", err)
	}
}

// TestWithHTTPClient 注入自定义底层客户端；nil 不覆盖默认。
func TestWithHTTPClient(t *testing.T) {
	custom := apiclient.NewClient(nil)
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{"http://x?"}}, nil, WithHTTPClient(custom))
	if api.client != custom {
		t.Fatal("WithHTTPClient 未注入")
	}
	// nil 视作未设置，保留默认
	api2 := NewRestApi(Config{ServiceName: "s", Urls: []string{"http://x?"}}, nil, WithHTTPClient(nil))
	if api2.client == nil {
		t.Fatal("nil 不应清空默认 client")
	}
}

// TestSetRestIns_NilGuards SetRestIns 对空 key / nil impl 静默忽略。
func TestSetRestIns_NilGuards(t *testing.T) {
	m := NewRestApiMgr()
	m.SetRestIns("", &RestApi{}) // 空 key：忽略
	m.SetRestIns("x", nil)       // nil impl：忽略
	if m.Count() != 0 {
		t.Fatalf("空 key/nil impl 应被忽略, Count=%d", m.Count())
	}
}

// TestNewUrlPool_Empty 空输入返回 nil（兜底，正常构造时已拦截）。
func TestNewUrlPool_Empty(t *testing.T) {
	if newUrlPool(nil) != nil {
		t.Fatal("空 urls 应返回 nil")
	}
}

// TestHeaderPassthrough header 透传到服务端（Authorization / Content-Type）。
func TestHeaderPassthrough(t *testing.T) {
	signer := testSigner()
	addr, _ := recordingServer(t, func(r *http.Request) (int, string) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type=%q", ct)
		}
		if a := r.Header.Get("Authorization"); a != "tok" {
			t.Errorf("Authorization=%q", a)
		}
		return 200, `{}`
	})
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{addr}, SignName: "default"},
		signMgrWith("default", signer))
	hdr := map[string]string{"Content-Type": "application/json", "Authorization": "tok"}
	if _, err := api.SignedPost(context.Background(), 0, nil, []byte("{}"), hdr); err != nil {
		t.Fatalf("SignedPost: %v", err)
	}
}

// ---------- Benchmark ----------

func BenchmarkSignedPost_Parallel(b *testing.B) {
	signer := testSigner()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{srv.URL + "?"}, SignName: "default"},
		signMgrWith("default", signer))
	body := []byte(`{"a":1}`)
	q := map[string]string{"user": "alice"}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = api.SignedPost(context.Background(), 0, q, body, nil)
		}
	})
}

func BenchmarkGet_Parallel(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()
	api := NewRestApi(Config{ServiceName: "s", Urls: []string{srv.URL + "?"}}, nil)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = api.Get(context.Background(), 0, nil)
		}
	})
}

func BenchmarkUrlPool_Pick(b *testing.B) {
	p := newUrlPool([]string{"a", "b", "c", "d", "e"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.pick(int64(i))
	}
}

func BenchmarkMgr_Get(b *testing.B) {
	m := NewRestApiMgr()
	m.SetRestIns("default", NewRestApi(Config{ServiceName: "default", Urls: []string{"http://x?"}}, nil))
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if m.GetRestIns() == nil {
				b.Fatal("nil")
			}
		}
	})
}
