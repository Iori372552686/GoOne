package http_client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestClientRejectsTooLargeResponse 验证 响应超过上限快速失败且返回
// ErrResponseTooLarge，内存不随输入无限增长（LimitReader(max+1) 探测）。
func TestClientRejectsTooLargeResponse(t *testing.T) {
	// 服务端返回超过上限的内容。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 100))
	}))
	defer srv.Close()

	c := NewClient(nil, WithMaxResponseBytes(32))
	body, err := c.DoRequest(context.Background(), "GET", srv.URL, "", nil)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got err=%v bodyLen=%d", err, len(body))
	}
}

// TestClientReadsNormalResponse 验证正常响应被完整读取。
func TestClientReadsNormalResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	c := NewClient(nil, WithMaxResponseBytes(1024))
	body, err := c.DoRequest(context.Background(), "GET", srv.URL, "", nil)
	if err != nil {
		t.Fatalf("DoRequest: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("body=%q want hello", string(body))
	}
}

// TestClientNon2xxReturnsError 验证非 2xx 返回 error（body 仍受上限保护）。
func TestClientNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := NewClient(nil, WithMaxResponseBytes(1024))
	if _, err := c.DoRequest(context.Background(), "GET", srv.URL, "", nil); err == nil {
		t.Fatal("expected error for 500")
	}
}

// TestClientContextCancelAbortsRequest 验证 请求可被 context 取消。
func TestClientContextCancelAbortsRequest(t *testing.T) {
	// 服务端慢响应，给取消留窗口。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	c := NewClient(nil, WithMaxResponseBytes(1024))
	if _, err := c.DoRequest(ctx, "GET", srv.URL, "", nil); err == nil {
		t.Fatal("expected error due to context cancel")
	}
}

// TestClientNewRequestErrorCheckedBeforeHeader 验证 NewRequest 失败时
// 不 panic（历史代码先写 Header 后查 error）。用非法 URL 触发 NewRequest error。
func TestClientNewRequestErrorCheckedBeforeHeader(t *testing.T) {
	c := NewClient(nil)
	if _, err := c.DoRequest(context.Background(), "GET", "http://%zz", "", map[string]string{"X": "Y"}); err == nil {
		t.Fatal("expected NewRequest error for invalid URL")
	}
}

// TestClientHeadersAppliedAfterErrorCheck 验证 Header 在 NewRequest 成功后写入，
// Authorization/Content-Type 正确设置（字段白名单日志之外的转发不丢）。
func TestClientHeadersAppliedAfterErrorCheck(t *testing.T) {
	got := make(http.Header)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer srv.Close()

	c := NewClient(nil)
	if _, err := c.DoRequest(context.Background(), "POST", srv.URL, "k=v",
		map[string]string{"Authorization": "Bearer x", "Content-Type": "application/json"}); err != nil {
		t.Fatalf("DoRequest: %v", err)
	}
	if got.Get("Authorization") != "Bearer x" {
		t.Fatalf("Authorization header lost: %v", got.Get("Authorization"))
	}
	if got.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type header wrong: %v", got.Get("Content-Type"))
	}
}

// TestClientDefaultMaxBytesBoundsLargeInput 验证未配置时默认上限生效：
// 大响应不会触发无限缓冲（LimitReader 截断后 ErrResponseTooLarge）。
func TestClientDefaultMaxBytesBoundsLargeInput(t *testing.T) {
	// 返回超过 4MiB 的内容。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 分块写，避免一次性分配过大切片阻塞测试。
		chunk := make([]byte, 64*1024)
		for i := 0; i < 100; i++ { // 6.4 MiB
			_, _ = w.Write(chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer srv.Close()

	c := NewClient(nil) // 默认 4 MiB
	start := time.Now()
	if _, err := c.DoRequest(context.Background(), "GET", srv.URL, "", nil); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge with default limit, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("limit reader should short-circuit fast, took %v", elapsed)
	}
}

// 防止 io import 被裁掉（若后续重构移除使用，再清理）。
var _ = io.EOF
