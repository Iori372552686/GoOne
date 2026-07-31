package plug

import (
	"net/http"
	"sync/atomic"
	"testing"
)

// TestNotifyDisabledWithoutEndpoint 验证未配置通知地址时 UploadFatalToDingHook 不得发起
// 任何网络请求，也不得回退到任何内置地址。这是 的安全不变量。
func TestNotifyDisabledWithoutEndpoint(t *testing.T) {
	// 重置为干净状态：未配置地址。
	ConfigureFatalHook("")

	var calls int32
	reset := swapHTTPDo(func(req *http.Request) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	t.Cleanup(reset)

	UploadFatalToDingHook("fatal text")

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected 0 network calls when endpoint unset, got %d", got)
	}
}

// TestNotifyFiresWhenConfigured 验证配置地址后通知会发起一次请求到该地址。
func TestNotifyFiresWhenConfigured(t *testing.T) {
	const addr = "https://example.invalid/robot/send"
	ConfigureFatalHook(addr)
	t.Cleanup(func() { ConfigureFatalHook("") })

	var (
		calls int32
		got   string
	)
	reset := swapHTTPDo(func(req *http.Request) error {
		atomic.AddInt32(&calls, 1)
		got = req.URL.String()
		return nil
	})
	t.Cleanup(reset)

	UploadFatalToDingHook("fatal text")

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 network call, got %d", got)
	}
	if got != addr {
		t.Fatalf("request sent to %q, want %q", got, addr)
	}
}

// swapHTTPDo 替换包级 httpDo，返回恢复函数。避免并发用例互相污染。
func swapHTTPDo(fn func(*http.Request) error) func() {
	fatalHookMu.Lock()
	orig := httpDo
	httpDo = fn
	fatalHookMu.Unlock()
	return func() {
		fatalHookMu.Lock()
		httpDo = orig
		fatalHookMu.Unlock()
	}
}
