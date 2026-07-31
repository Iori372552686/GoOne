package ssrpc

import (
	"errors"
	"testing"

	"github.com/golang/protobuf/proto"
)

// fakeLimiter 是测试用的 InflightLimiter，可控在途计数与拒绝阈值。
type fakeLimiter struct {
	inflight int64
	rejectAt int64 // 达到此值时 InflightWouldReject 返回 true
}

func (f *fakeLimiter) IncInflight() int64 { f.inflight++; return f.inflight }
func (f *fakeLimiter) DecInflight() int64 { f.inflight--; return f.inflight }
func (f *fakeLimiter) InflightWouldReject(max int) bool {
	if max <= 0 {
		return false
	}
	return f.inflight >= int64(max)
}

// noopHandler 总是成功返回。
func noopHandler(_ *Context, _ proto.Message) (proto.Message, error) {
	return nil, nil
}

// TestAdmissionMiddlewareAdmitsWhenBelowLimit 验证未达上限时放行。
func TestAdmissionMiddlewareAdmitsWhenBelowLimit(t *testing.T) {
	lim := &fakeLimiter{rejectAt: 5}
	mw := AdmissionMiddleware(lim, 5, nil)
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	if _, err := h(ctx, nil); err != nil {
		t.Fatalf("expected admit, got err: %v", err)
	}
}

// TestAdmissionMiddlewareRejectsWhenOverloaded 验证达上限时拒绝 ErrOverloaded。
func TestAdmissionMiddlewareRejectsWhenOverloaded(t *testing.T) {
	lim := &fakeLimiter{inflight: 5} // 已达上限
	mw := AdmissionMiddleware(lim, 5, nil)
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	_, err := h(ctx, nil)
	if !errors.Is(err, ErrOverloaded) {
		t.Fatalf("expected ErrOverloaded, got: %v", err)
	}
}

// TestAdmissionMiddlewareNilLimiterPassthrough 验证 nil limiter 直通。
func TestAdmissionMiddlewareNilLimiterPassthrough(t *testing.T) {
	mw := AdmissionMiddleware(nil, 5, nil)
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	if _, err := h(ctx, nil); err != nil {
		t.Fatalf("nil limiter must passthrough, got err: %v", err)
	}
}

// TestAdmissionMiddlewareZeroMaxPassthrough 验证 max<=0 直通。
func TestAdmissionMiddlewareZeroMaxPassthrough(t *testing.T) {
	lim := &fakeLimiter{inflight: 999}
	mw := AdmissionMiddleware(lim, 0, nil) // max=0 不限
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	if _, err := h(ctx, nil); err != nil {
		t.Fatalf("max=0 must passthrough, got err: %v", err)
	}
}

// TestAdmissionMiddlewarePerMethodOverride 验证 per-method 覆盖全局。
func TestAdmissionMiddlewarePerMethodOverride(t *testing.T) {
	lim := &fakeLimiter{inflight: 3}
	// 全局 max=10，但 Svc.Do 覆盖为 3。
	mw := AdmissionMiddleware(lim, 10, map[string]int{"Svc.Do": 3})
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	_, err := h(ctx, nil)
	if !errors.Is(err, ErrOverloaded) {
		t.Fatalf("per-method override should reject at 3, got: %v", err)
	}
}

// TestAdmissionMiddlewareIncDecBalanced 验证放行后计数 Inc/Dec 平衡。
func TestAdmissionMiddlewareIncDecBalanced(t *testing.T) {
	lim := &fakeLimiter{}
	mw := AdmissionMiddleware(lim, 10, nil)
	h := mw(noopHandler)
	ctx := &Context{Method: "Svc.Do"}
	for i := 0; i < 5; i++ {
		if _, err := h(ctx, nil); err != nil {
			t.Fatalf("unexpected reject at %d: %v", i, err)
		}
	}
	// 5 个请求串行处理完，inflight 应归零（Inc 后 Dec）。
	if lim.inflight != 0 {
		t.Fatalf("inflight not balanced, got %d, want 0", lim.inflight)
	}
}
