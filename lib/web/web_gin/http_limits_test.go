package web_gin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMaxBodyBytesMiddlewareRejectsOversizedRequest 验证 MaxBodyBytes>0 时，
// 超过上限的请求体在业务读取前被拒绝（返回 413），内存不随输入无限增长。
//
// 中间件实现即用 http.MaxBytesReader 套裹 r.Body；handler 读取超限 Body 时得到
// http.MaxBytesError，本测试直接验证该语义与 413 映射。
func TestMaxBodyBytesMiddlewareRejectsOversizedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	const max int64 = 16
	r.Use(maxBodyBytesMiddleware(max))
	readErr := make(chan error, 1)
	r.POST("/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		readErr <- err
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	// 超限请求：1024 字节远超 16。
	req := httptest.NewRequest("POST", "/upload", strings.NewReader(strings.Repeat("x", 1024)))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d", w.Code)
	}
	if err := <-readErr; err == nil {
		t.Fatal("expected handler to see a body read error")
	}
}

// TestMaxBodyBytesMiddlewareAllowsUnderLimit 验证 未超限的请求正常通过。
func TestMaxBodyBytesMiddlewareAllowsUnderLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	const max int64 = 64
	r.Use(maxBodyBytesMiddleware(max))
	r.POST("/upload", func(c *gin.Context) {
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "text/plain", data)
	})

	req := httptest.NewRequest("POST", "/upload", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for normal body, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("body=%q want hello", w.Body.String())
	}
}
