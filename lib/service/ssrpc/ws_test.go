package ssrpc

import (
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// ---------------------------------------------------------------------------
// WrapWS tests
// ---------------------------------------------------------------------------

func TestWrapWS_SetsTransportWS(t *testing.T) {
	var gotTransport Transport
	desc := MethodDesc{Cmd: 100, Name: "test.Login"}
	h := WrapWS(desc, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			gotTransport = ctx.Transport
			return nil, nil
		},
	)

	ic := &fakeIContext{uid: 1}
	code := h(ic, nil)
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	if gotTransport != TransportWS {
		t.Fatalf("expected transport %q, got %q", TransportWS, gotTransport)
	}
}

type fakeTransportIContext struct {
	fakeIContext
	transport Transport
}

func (f *fakeTransportIContext) SSRPCTransport() Transport { return f.transport }

func TestWrapWS_UsesTransportHint(t *testing.T) {
	var gotTransport Transport
	desc := MethodDesc{Cmd: 101, Name: "test.LoginTCP"}
	h := WrapWS(desc, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			gotTransport = ctx.Transport
			return nil, nil
		},
	)

	ic := &fakeTransportIContext{
		fakeIContext: fakeIContext{uid: 9},
		transport:    TransportTCP,
	}
	code := h(ic, nil)
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	if gotTransport != TransportTCP {
		t.Fatalf("expected transport %q, got %q", TransportTCP, gotTransport)
	}
}

func TestWrapWS_RunsMiddleware(t *testing.T) {
	var order []string
	mw1 := func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			order = append(order, "mw1-before")
			rsp, err := next(ctx, req)
			order = append(order, "mw1-after")
			return rsp, err
		}
	}
	mw2 := func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			order = append(order, "mw2-before")
			rsp, err := next(ctx, req)
			order = append(order, "mw2-after")
			return rsp, err
		}
	}

	desc := MethodDesc{Cmd: 200, Name: "test.MW"}
	h := WrapWS(desc, []Middleware{mw1, mw2},
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			order = append(order, "handler")
			return nil, nil
		},
	)

	ic := &fakeIContext{uid: 2}
	code := h(ic, nil)
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("middleware order mismatch: got %v", order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("middleware order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestWrapWS_PropagatesMethodDesc(t *testing.T) {
	var gotMethod string
	var gotAuth, gotSign bool
	desc := MethodDesc{
		Cmd:       300,
		Name:      "test.Desc",
		Auth:      true,
		Sign:      true,
		TraceTags: map[string]string{"env": "test"},
	}
	h := WrapWS(desc, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			gotMethod = ctx.Method
			gotAuth = ctx.AuthRequired
			gotSign = ctx.SignRequired
			return nil, nil
		},
	)

	ic := &fakeIContext{uid: 3}
	h(ic, nil)
	if gotMethod != "test.Desc" {
		t.Fatalf("method = %q, want %q", gotMethod, "test.Desc")
	}
	if !gotAuth || !gotSign {
		t.Fatalf("auth=%v sign=%v, want both true", gotAuth, gotSign)
	}
}

func TestWrapWS_NilContext_ReturnsInternal(t *testing.T) {
	desc := MethodDesc{Cmd: 400}
	h := WrapWS(desc, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			t.Fatal("handler should not be called")
			return nil, nil
		},
	)
	code := h(nil, nil)
	if code != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected ERR_INTERNAL for nil context, got %v", code)
	}
}

func TestWrapWS_ErrorMapping(t *testing.T) {
	desc := MethodDesc{Cmd: 500, Name: "test.Err"}
	h := WrapWS(desc, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			return nil, E(g1_protocol.ErrorCode_ERR_ARGV, "bad param")
		},
	)

	ic := &fakeIContext{uid: 4}
	code := h(ic, nil)
	if code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("expected ERR_ARGV, got %v", code)
	}
}

// ---------------------------------------------------------------------------
// Dispatcher WS tests
// ---------------------------------------------------------------------------

func TestDispatcher_RegisterWS_And_DispatchWS(t *testing.T) {
	d := NewDispatcher()
	called := false

	d.RegisterWS(100, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		called = true
		return g1_protocol.ErrorCode_ERR_OK
	})

	ic := &fakeIContext{uid: 10}
	code, handled := d.DispatchWS(ic, 100, nil)
	if !handled {
		t.Fatal("expected handled=true")
	}
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestDispatcher_DispatchWS_Unregistered(t *testing.T) {
	d := NewDispatcher()

	ic := &fakeIContext{uid: 11}
	_, handled := d.DispatchWS(ic, 999, nil)
	if handled {
		t.Fatal("expected handled=false for unregistered cmd")
	}
}

func TestDispatcher_DispatchWS_NilDispatcher(t *testing.T) {
	var d *Dispatcher
	_, handled := d.DispatchWS(&fakeIContext{}, 100, nil)
	if handled {
		t.Fatal("expected handled=false for nil dispatcher")
	}
}

func TestDispatcher_RegisterWS_NilHandler(t *testing.T) {
	d := NewDispatcher()
	d.RegisterWS(100, nil) // should be a no-op

	_, handled := d.DispatchWS(&fakeIContext{}, 100, nil)
	if handled {
		t.Fatal("nil handler should not have been registered")
	}
}

func TestDispatcher_RegisterWS_NilDispatcher(t *testing.T) {
	var d *Dispatcher
	// Should not panic.
	d.RegisterWS(100, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	})
}

func TestDispatcher_WS_DoesNotInterfereWithCmd(t *testing.T) {
	d := NewDispatcher()

	wsCalled := false
	cmdCalled := false

	d.RegisterWS(100, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		wsCalled = true
		return g1_protocol.ErrorCode_ERR_OK
	})
	d.RegisterCmd(g1_protocol.CMD(100), func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		cmdCalled = true
		return g1_protocol.ErrorCode_ERR_OK
	})

	ic := &fakeIContext{uid: 12}

	// Dispatch WS should only call the WS handler
	d.DispatchWS(ic, 100, nil)
	if !wsCalled {
		t.Fatal("WS handler should have been called")
	}
	if cmdCalled {
		t.Fatal("cmd handler should not have been called by DispatchWS")
	}
}
