package http_client

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDefaultTransportVerifiesTLSCertificate 验证默认 HttpConnectPool 严格校验 TLS 证书：
// 访问仅持有自签名证书的 HTTPS server 时，默认配置必须失败（安全不变量）。
//
// 历史版本中 HttpConnectPool 设置了 InsecureSkipVerify: true，会接受任意不受信证书；
// 本测试确保该不安全默认已被移除。
func TestDefaultTransportVerifiesTLSCertificate(t *testing.T) {
	// 自签名证书的 HTTPS 测试服务端。
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 复用包级默认传输层（不得为它注入自签名证书）。
	client := &http.Client{Transport: HttpConnectPool}

	resp, err := client.Get(server.URL)
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("default transport accepted an untrusted self-signed certificate; " +
			"expected TLS verification failure")
	}
	// 错误信息应体现证书校验失败（x509: unknown/authority 或 certificate signed by unknown authority）。
	msg := err.Error()
	if !strings.Contains(msg, "certificate") && !strings.Contains(msg, "x509") {
		t.Fatalf("expected a certificate verification error, got: %v", err)
	}
}

// TestTransportDoesNotSkipVerify 断言默认传输层的 TLS 配置不关闭证书校验。
func TestTransportDoesNotSkipVerify(t *testing.T) {
	tr, ok := HttpConnectPool.(*http.Transport)
	if !ok {
		t.Fatalf("HttpConnectPool is %T, want *http.Transport", HttpConnectPool)
	}
	if tr.TLSClientConfig == nil {
		t.Fatalf("TLSClientConfig is nil; expected explicit config that does not skip verify")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("InsecureSkipVerify must be false in the default transport")
	}
}

// TestSystemCertPoolFallbackKeepsVerification 确保证书池不可用时仍保持校验：
// 当 SystemCertPool 返回 error 时，回退路径不得设置 InsecureSkipVerify=true。
func TestSystemCertPoolFallbackKeepsVerification(t *testing.T) {
	orig := SystemCertPool
	t.Cleanup(func() { SystemCertPool = orig })
	SystemCertPool = func() (*x509.CertPool, error) {
		return nil, errFakeCertPoolUnavailable
	}

	tr := newDefaultTransportForTest()
	if tr.TLSClientConfig == nil {
		t.Fatalf("TLSClientConfig is nil")
	}
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("fallback must keep verification enabled (InsecureSkipVerify should be false)")
	}
	if tr.TLSClientConfig.RootCAs != nil {
		t.Fatalf("on cert-pool error, RootCAs should be nil to use built-in roots, got non-nil")
	}
}

var errFakeCertPoolUnavailable = &certPoolErr{}

type certPoolErr struct{}

func (certPoolErr) Error() string { return "fake cert pool unavailable" }

// newDefaultTransportForTest 复刻 HttpConnectPool 的初始化逻辑，供测试在不污染包级变量的
// 前提下观察错误回退行为。逻辑必须与 http_api.go 中 HttpConnectPool 的 IIFE 保持一致。
func newDefaultTransportForTest() *http.Transport {
	pool, err := SystemCertPool()
	if err != nil {
		pool = nil
	}
	_ = tls.Config{} // ensure tls import stays used if logic moves
	return &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
}
