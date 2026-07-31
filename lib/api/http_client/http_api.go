package http_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"time"
)

// HttpConnectPool 是包级默认 HTTP 传输层。
//
// 安全说明：默认严格校验 TLS 证书（使用系统证书池），不再设置
// InsecureSkipVerify: true。仅测试专用构造器可显式跳过校验，生产代码不得回退。
var HttpConnectPool http.RoundTripper = func() http.RoundTripper {
	pool, err := SystemCertPool()
	if err != nil {
		// 系统证书池不可用时退回 nil，由 crypto/tls 使用其内置根证书校验逻辑，
		// 仍然校验证书链，绝不关闭校验。
		pool = nil
	}
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          1000,
		IdleConnTimeout:       60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}()

// SystemCertPool 返回系统证书池。抽为变量以便测试替换。
var SystemCertPool = func() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}

// defaultClient 是旧包级函数共享的统一 Client。
// 所有 Deprecated 函数内部委托它，获得 context 优先、LimitReader 上限与
// NewRequest error 先检的安全语义。
var defaultClient = NewClient(nil)

// Deprecated: 请改用 Client.DoRequest。本函数保留以兼容旧调用方。
func GetRequest(url string) ([]byte, error) {
	return HttpRequest("GET", url, "")
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func HttpRequestByHeader(method, url, requestBody string, header map[string]string) ([]byte, error) {
	if header == nil {
		header = map[string]string{}
	}
	if _, ok := header["Content-Type"]; !ok {
		header["Content-Type"] = "application/x-www-form-urlencoded"
	}
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	return defaultClient.DoRequest(ctx, method, url, requestBody, header)
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func HttpRequest(method, url, requestBody string) ([]byte, error) {
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	return defaultClient.DoRequest(ctx, method, url, requestBody, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func TokenHttpRequest(method string, value url.Values, url, token string) ([]byte, error) {
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	return defaultClient.DoRequest(ctx, method, url, value.Encode(), map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": token,
	})
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func HttpGetRequest(urlstr, reqBody string) ([]byte, error) {
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	return defaultClient.DoRequest(ctx, "GET", urlstr, reqBody, nil)
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func HttpPostRequest(urlstr, msgbody string) ([]byte, error) {
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	return defaultClient.DoRequest(ctx, "POST", urlstr, msgbody, map[string]string{
		"Content-Type": "application/json",
	})
}

// Deprecated: 请改用 Client.DoRequest（支持 context 与响应上限）。
func HeaderHttpPostRequest(urlstr, msgbody string, headers *map[string]string) ([]byte, error) {
	hdr := map[string]string{}
	if headers != nil {
		for k, v := range *headers {
			hdr[k] = v
		}
	}
	ctx, cancel := defaultRequestCtx()
	defer cancel()
	// DoRequest 对 2xx 返回 (body, nil)，其余返回 (body, error)，与旧函数
	// 「非 200/201 视为错误但回传 body」的语义兼容。
	return defaultClient.DoRequest(ctx, "POST", urlstr, msgbody, hdr)
}

// defaultRequestCtx 提供旧函数的默认超时（与历史 8s 对齐），使旧调用方平滑获得
// context 取消能力。
func defaultRequestCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 8*time.Second)
}
