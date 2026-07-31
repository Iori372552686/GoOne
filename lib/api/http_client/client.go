package http_client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrResponseTooLarge 在响应体超过 Client 配置的最大字节数时返回。
// 用 LimitReader(max+1) 探测：读到的字节数 > max 即判定越界，避免内存随输入无限增长。
var ErrResponseTooLarge = errors.New("http_client: response body too large")

// DefaultMaxResponseBytes 是未显式配置时的响应体上限。
// 4 MiB 覆盖绝大多数 JSON/protobuf 配置响应；超出按错误处理而非无限缓冲。
const DefaultMaxResponseBytes int64 = 4 << 20

// Client 是统一的出站 HTTP 客户端（取代散落的 *http.Client 直构）。
//
// 设计要点：
//   - 构造时注入 *http.Client（含 Transport/TLS/Timeout），禁止调用方各自复制 Transport；
//   - 所有请求经 NewRequestWithContext，支持 context 取消与超时；
//   - 响应体用 io.LimitReader(max+1) 读取，超过 MaxResponseBytes 返回 ErrResponseTooLarge；
//   - 仅当 StatusCode 为 2xx 时返回 body，否则返回带状态码的 error（body 仍受上限保护）。
//
// 旧的包级函数（GetRequest/HttpRequest/...）保留为 Deprecated 薄封装，内部委托本 Client，
// 行为兼容，一个稳定版本后删除。
type Client struct {
	httpClient       *http.Client
	maxResponseBytes int64
	defaultMediaType string
}

// Option 配置 Client。
type Option func(*Client)

// WithMaxResponseBytes 设置单次响应体上限。
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxResponseBytes = n
		}
	}
}

// WithDefaultMediaType 设置未显式指定 Content-Type 时的默认值（兼容旧行为）。
func WithDefaultMediaType(mt string) Option {
	return func(c *Client) {
		if mt != "" {
			c.defaultMediaType = mt
		}
	}
}

// NewClient 构造统一 Client。httpClient 为 nil 时使用包级默认传输层与超时。
func NewClient(httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: HttpConnectPool,
			Timeout:   8 * time.Second,
		}
	}
	c := &Client{
		httpClient:       httpClient,
		maxResponseBytes: DefaultMaxResponseBytes,
		defaultMediaType: "application/x-www-form-urlencoded",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// DoRequest 执行一次 HTTP 请求（核心方法）。
//
// ctx 控制：nil 时用 context.Background（但建议总是传入带超时的 ctx）。
// headers 中的键值在 NewRequest 成功后才写入 Header（修复历史「先写 Header 后查 error」缺陷）。
// body 可为空字符串。
//
// 返回：2xx 时 (body, nil)；非 2xx 时 (body, error)，body 仍受上限保护且可能非空。
func (c *Client) DoRequest(ctx context.Context, method, url, body string, headers map[string]string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		// 先检查 error，再访问 req 字段（历史代码在 error 后仍 Set Header）。
		return nil, fmt.Errorf("http_client: build request: %w", err)
	}

	contentType := headers["Content-Type"]
	if contentType == "" {
		contentType = c.defaultMediaType
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		if k == "Content-Type" {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// LimitReader(max+1) 探测越界，内存不随输入无限增长。
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxResponseBytes {
		return nil, fmt.Errorf("%w (limit=%d)", ErrResponseTooLarge, c.maxResponseBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, fmt.Errorf("http_client: status %d for %s", resp.StatusCode, url)
	}
	return data, nil
}
