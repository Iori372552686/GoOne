package ssrpc

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/gin-gonic/gin"
)

// dummyCmdHandler is a non-nil cmd handler usable in tests.
func dummyCmdHandler() cmd_handler.CmdHandlerFunc {
	return func(cmd_handler.IContext, []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	}
}

func dummyHTTPHandler() gin.HandlerFunc {
	return func(*gin.Context) {}
}

func dummyUnary() (GRPCUnaryHandler, func() any) {
	return func(context.Context, any) (any, error) { return nil, nil }, func() any { return new(int) }
}

func TestRegistryRejectsEmptyService(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("", Binding{Kind: BindingCMD, CMD: 1, CmdHandler: dummyCmdHandler()}); !errors.Is(err, ErrEmptyService) {
		t.Fatalf("expected ErrEmptyService, got %v", err)
	}
}

func TestRegistryRejectsNilHandler(t *testing.T) {
	r := NewRegistry()
	err := r.Register("svc", Binding{Kind: BindingCMD, CMD: 1})
	if !errors.Is(err, ErrNilHandler) {
		t.Fatalf("expected ErrNilHandler, got %v", err)
	}
}

func TestRegistryRejectsInvalidBinding(t *testing.T) {
	r := NewRegistry()
	err := r.Register("svc", Binding{Kind: BindingInvalid})
	if !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("expected ErrInvalidBinding, got %v", err)
	}
	// HTTP with empty path is invalid.
	err = r.Register("svc", Binding{Kind: BindingHTTP, HTTPPath: "  ", HTTPHandler: dummyHTTPHandler()})
	if !errors.Is(err, ErrInvalidBinding) {
		t.Fatalf("expected ErrInvalidBinding for empty path, got %v", err)
	}
}

func TestRegistryDuplicateCMDWithinBatchRejected(t *testing.T) {
	r := NewRegistry()
	b := Binding{Kind: BindingCMD, CMD: 42, CmdHandler: dummyCmdHandler()}
	err := r.Register("svc", b, b)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !errors.Is(err, ErrDuplicateBinding) {
		t.Fatalf("expected ErrDuplicateBinding, got %v", err)
	}
	// Whole batch rejected: state unchanged, nothing registered.
	if got := len(r.keys); got != 0 {
		t.Fatalf("expected 0 keys after rejected batch, got %d", got)
	}
}

func TestRegistryDuplicateCMDAcrossBatchesRejected(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("svc", Binding{Kind: BindingCMD, CMD: 7, CmdHandler: dummyCmdHandler()}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	err := r.Register("svc", Binding{Kind: BindingCMD, CMD: 7, CmdHandler: dummyCmdHandler()})
	if !errors.Is(err, ErrDuplicateBinding) {
		t.Fatalf("expected ErrDuplicateBinding across batches, got %v", err)
	}
}

func TestRegistryDifferentKindsDoNotCollide(t *testing.T) {
	// CMD 5 and WS cmd 5 share the integer but live in different key spaces.
	r := NewRegistry()
	if err := r.Register("svc",
		Binding{Kind: BindingCMD, CMD: 5, CmdHandler: dummyCmdHandler()},
		Binding{Kind: BindingWS, CMD: 5, CmdHandler: dummyCmdHandler()},
	); err != nil {
		t.Fatalf("expected distinct kinds to coexist, got %v", err)
	}
}

func TestRegistryDuplicateHTTPAndGRPCRejected(t *testing.T) {
	r := NewRegistry()
	h := Binding{Kind: BindingHTTP, HTTPMethod: "get", HTTPPath: "/x", HTTPHandler: dummyHTTPHandler()}
	if err := r.Register("svc", h); err != nil {
		t.Fatalf("first http: %v", err)
	}
	// Method normalization: GET vs get must still collide.
	err := r.Register("svc", Binding{Kind: BindingHTTP, HTTPMethod: "GET", HTTPPath: "/x", HTTPHandler: dummyHTTPHandler()})
	if !errors.Is(err, ErrDuplicateBinding) {
		t.Fatalf("expected http collision after method normalization, got %v", err)
	}

	unary, req := dummyUnary()
	gb := Binding{Kind: BindingGRPCUnary, GRPCService: "svc.S", GRPCMethod: "Do", UnaryHandler: unary, ReqFactory: req}
	if err := r.Register("svc", gb); err != nil {
		t.Fatalf("first grpc: %v", err)
	}
	err = r.Register("svc", Binding{Kind: BindingGRPCUnary, GRPCService: "svc.S", GRPCMethod: "Do", UnaryHandler: unary, ReqFactory: req})
	if !errors.Is(err, ErrDuplicateBinding) {
		t.Fatalf("expected grpc collision, got %v", err)
	}
}

func TestRegistrySealProducesSealedDispatcherAndLocksRegistration(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("svc",
		Binding{Kind: BindingCMD, CMD: 1, CmdHandler: dummyCmdHandler()},
		Binding{Kind: BindingWS, CMD: 1, CmdHandler: dummyCmdHandler()},
		Binding{Kind: BindingHTTP, HTTPMethod: "post", HTTPPath: "/hi", HTTPHandler: dummyHTTPHandler()},
	); err != nil {
		t.Fatalf("register: %v", err)
	}
	d, err := r.Seal()
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !d.Sealed() {
		t.Fatal("expected sealed dispatcher")
	}
	// Post-seal registration rejected.
	if err := r.Register("svc", Binding{Kind: BindingCMD, CMD: 99, CmdHandler: dummyCmdHandler()}); !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("expected ErrRegistrySealed, got %v", err)
	}
}

func TestSealedDispatcherDispatchWSLockFree(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("svc", Binding{Kind: BindingWS, CMD: 11, CmdHandler: dummyCmdHandler()}); err != nil {
		t.Fatalf("register: %v", err)
	}
	d, err := r.Seal()
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// DispatchWS on a sealed dispatcher must find the handler and run it.
	code, ok := d.DispatchWS(nil, 11, nil)
	if !ok {
		t.Fatal("expected handler found")
	}
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	// Unknown cmd returns not-found, not panic.
	_, ok = d.DispatchWS(nil, 999, nil)
	if ok {
		t.Fatal("expected not-found for unregistered cmd")
	}
}

func TestSealedDispatcherConcurrentDispatch(t *testing.T) {
	// Even on a non-race local run, this exercises the lock-free read path
	// under concurrent goroutines to catch obvious data races in code review.
	r := NewRegistry()
	var bindings []Binding
	for i := 0; i < 50; i++ {
		bindings = append(bindings, Binding{Kind: BindingWS, CMD: g1_protocol.CMD(100 + i), CmdHandler: dummyCmdHandler()})
	}
	if err := r.Register("svc", bindings...); err != nil {
		t.Fatalf("register: %v", err)
	}
	d, err := r.Seal()
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				_, _ = d.DispatchWS(nil, uint32(100+(i%50)), nil)
			}
		}()
	}
	wg.Wait()
}

func TestRegisterCmdERejectsAfterSeal(t *testing.T) {
	d := NewDispatcher()
	d.Seal()
	if err := d.RegisterCmdE(1, dummyCmdHandler()); !errors.Is(err, ErrDispatcherSealed) {
		t.Fatalf("expected ErrDispatcherSealed, got %v", err)
	}
}

func TestRegisterCmdERejectsNilHandler(t *testing.T) {
	d := NewDispatcher()
	if err := d.RegisterCmdE(1, nil); !errors.Is(err, ErrNilHandler) {
		t.Fatalf("expected ErrNilHandler, got %v", err)
	}
}

func TestLegacyRegisterCmdIsNoOpAfterSeal(t *testing.T) {
	// The legacy void-return method must not panic after Seal (generated code
	// calls it). It should log and ignore.
	d := NewDispatcher()
	d.Seal()
	d.RegisterCmd(1, dummyCmdHandler()) // must not panic
	if _, ok := d.cmdHandlers[1]; ok {
		t.Fatal("sealed dispatcher must not accept legacy registration")
	}
}

func TestRegistryBindingKeyRoundTrip(t *testing.T) {
	// Method case normalization: "get" and "GET" must produce the same key,
	// while distinct methods (GET vs POST) must NOT collide.
	cases := []struct {
		b    Binding
		want string
	}{
		{Binding{Kind: BindingCMD, CMD: 3}, "cmd:3"},
		{Binding{Kind: BindingWS, CMD: 3}, "ws:3"},
		{Binding{Kind: BindingHTTP, HTTPMethod: "get", HTTPPath: "/a"}, "http:GET:/a"},
		{Binding{Kind: BindingHTTP, HTTPMethod: "GET", HTTPPath: "/a"}, "http:GET:/a"}, // same as above
		{Binding{Kind: BindingHTTP, HTTPMethod: "POST", HTTPPath: "/a"}, "http:POST:/a"}, // distinct method
	}
	seen := make(map[string]int)
	for _, c := range cases {
		k, ok := c.b.key()
		if !ok {
			t.Fatalf("expected key for %+v", c.b)
		}
		if k != c.want {
			t.Fatalf("key mismatch: want %q, got %q (binding %+v)", c.want, k, c.b)
		}
		seen[k]++
	}
	if seen["http:GET:/a"] != 2 {
		t.Fatalf("expected get/GET to normalize to same key, got %v", seen)
	}
	if seen["http:POST:/a"] != 1 {
		t.Fatalf("expected distinct method to have own key, got %v", seen)
	}
}

// TestRegistrySealIsIdempotent 验证 Seal 真正幂等——二次调用返回同一个
// Dispatcher（历史返回 ErrRegistrySealed，与文档矛盾）。
func TestRegistrySealIsIdempotent(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("svc", Binding{Kind: BindingCMD, CMD: 42, CmdHandler: dummyCmdHandler()}); err != nil {
		t.Fatalf("register: %v", err)
	}
	d1, err := r.Seal()
	if err != nil {
		t.Fatalf("first seal: %v", err)
	}
	d2, err := r.Seal()
	if err != nil {
		t.Fatalf("second seal should be idempotent, got: %v", err)
	}
	if d1 != d2 {
		t.Fatal("幂等 Seal 应返回同一个 Dispatcher 实例")
	}
}

// TestRegistryRegisterNilReturnsErrNilRegistry 验证 nil 接收者返回
// ErrNilRegistry（历史返回 ErrNilDispatcher）。
func TestRegistryRegisterNilReturnsErrNilRegistry(t *testing.T) {
	var r *Registry
	if err := r.Register("svc"); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("期望 ErrNilRegistry，got %v", err)
	}
	if _, err := r.Seal(); !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("nil Seal 期望 ErrNilRegistry，got %v", err)
	}
}

// TestDispatcherRegisterBindings 验证 RegisterBindings 按 Kind 批量注册。
func TestDispatcherRegisterBindings(t *testing.T) {
	d := NewDispatcher()
	bindings := []Binding{
		{Kind: BindingCMD, CMD: 100, CmdHandler: dummyCmdHandler()},
		{Kind: BindingHTTP, HTTPMethod: "POST", HTTPPath: "/x", HTTPHandler: dummyHTTPHandler()},
		{Kind: BindingWS, CMD: 200, CmdHandler: dummyCmdHandler()},
	}
	if err := d.RegisterBindings(bindings...); err != nil {
		t.Fatalf("RegisterBindings: %v", err)
	}
	// 非法 binding（nil handler）应返回 error。
	if err := d.RegisterBindings(Binding{Kind: BindingCMD, CMD: 300, CmdHandler: nil}); !errors.Is(err, ErrNilHandler) {
		t.Fatalf("期望 ErrNilHandler，got %v", err)
	}
}
