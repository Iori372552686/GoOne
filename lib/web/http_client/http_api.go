// Package http_client 是 lib/api/http_client 的兼容代理。
//
// 历史上 lib/web/http_client 与 lib/api/http_client 是两份完全相同的重复实现，
// 导致 TLS 配置、超时、响应上限三处需同步维护。起统一到
// lib/api/http_client.Client；本包保留全部旧导出符号，内部委托 api 包，避免
// rest_api 等调用方一次性大改。
//
// Deprecated: 请改 import "github.com/Iori372552686/GoOne/lib/api/http_client"
// 并使用 Client.DoRequest。本包将在一个稳定版本后删除，不得新增使用。
package http_client

import (
	"net/url"

	apiclient "github.com/Iori372552686/GoOne/lib/api/http_client"
)

// HttpConnectPool 透传 api 包的默认传输层（单一 Transport 来源）。
var HttpConnectPool = apiclient.HttpConnectPool

// Deprecated: 见 apiclient.GetRequest。
func GetRequest(url string) ([]byte, error) {
	return apiclient.GetRequest(url)
}

// Deprecated: 见 apiclient.HttpRequestByHeader。
func HttpRequestByHeader(method, reqURL, requestBody string, header map[string]string) ([]byte, error) {
	return apiclient.HttpRequestByHeader(method, reqURL, requestBody, header)
}

// Deprecated: 见 apiclient.HttpRequest。
func HttpRequest(method, reqURL, requestBody string) ([]byte, error) {
	return apiclient.HttpRequest(method, reqURL, requestBody)
}

// Deprecated: 见 apiclient.TokenHttpRequest。
func TokenHttpRequest(method string, value url.Values, reqURL, token string) ([]byte, error) {
	return apiclient.TokenHttpRequest(method, value, reqURL, token)
}

// Deprecated: 见 apiclient.HttpGetRequest。
func HttpGetRequest(urlstr, reqBody string) ([]byte, error) {
	return apiclient.HttpGetRequest(urlstr, reqBody)
}

// Deprecated: 见 apiclient.HttpPostRequest。
func HttpPostRequest(urlstr, msgbody string) ([]byte, error) {
	return apiclient.HttpPostRequest(urlstr, msgbody)
}

// Deprecated: 见 apiclient.HeaderHttpPostRequest。
func HeaderHttpPostRequest(urlstr, msgbody string, headers *map[string]string) ([]byte, error) {
	return apiclient.HeaderHttpPostRequest(urlstr, msgbody, headers)
}
