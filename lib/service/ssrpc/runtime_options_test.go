package ssrpc

import (
	"context"
	"net/http"
	"strings"
 	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/metadata"
)

type fakeIContext struct {
	uid uint64
}

func (f *fakeIContext) Uid() uint64         { return f.uid }
func (f *fakeIContext) Zone() uint32        { return 0 }
func (f *fakeIContext) Rid() uint64         { return 0 }
func (f *fakeIContext) OriSrcBusId() uint32 { return 0 }
func (f *fakeIContext) Ip() uint32          { return 0 }
func (f *fakeIContext) Flag() uint32        { return 0 }
func (f *fakeIContext) ParseMsg(_ []byte, _ proto.Message) error {
	return nil
}
func (f *fakeIContext) CallMsgBySvrType(_ uint32, _ g1_protocol.CMD, _ proto.Message, _ proto.Message) error {
	panic("not used")
}
func (f *fakeIContext) CallMsgByRouter(_ uint32, _ uint64, _ g1_protocol.CMD, _ proto.Message, _ proto.Message) error {
	panic("not used")
}
func (f *fakeIContext) CallOtherMsgBySvrType(_ uint32, _ uint64, _ uint64, _ uint32, _ g1_protocol.CMD, _ proto.Message, _ proto.Message) error {
	panic("not used")
}
func (f *fakeIContext) SendMsgBack(_ proto.Message) {}
func (f *fakeIContext) SendMsgByServerType(_ uint32, _ g1_protocol.CMD, _ proto.Message) error {
	panic("not used")
}
func (f *fakeIContext) SendMsgByRouter(_ uint32, _ uint64, _ g1_protocol.CMD, _ proto.Message) error {
	panic("not used")
}
func (f *fakeIContext) Errorf(_ string, _ ...interface{})   {}
func (f *fakeIContext) Warningf(_ string, _ ...interface{}) {}
func (f *fakeIContext) Infof(_ string, _ ...interface{})    {}
func (f *fakeIContext) Debugf(_ string, _ ...interface{})   {}

var _ cmd_handler.IContext = (*fakeIContext)(nil)

type fakeTraceProvider struct {
	gotTags map[string]string
	finish  int
}

func (f *fakeTraceProvider) Start(_ *Context, tags map[string]string) func(err error) {
	f.gotTags = tags
	return func(err error) { f.finish++ }
}

type fakeLocker struct {
	lockCnt   int
	unlockCnt int
}

func (l *fakeLocker) Lock(_ uint64) func() {
	l.lockCnt++
	return func() { l.unlockCnt++ }
}

type fakeMCP struct {
	called int
}

func (m *fakeMCP) CallTool(_ string, _ any) (any, error) {
	m.called++
	return "ok", nil
}

type captureLogIContext struct {
	fakeIContext
	lastInfo string
}

func (c *captureLogIContext) Infof(format string, _ ...interface{}) {
	c.lastInfo = format
}

func TestTraceWith_MergesTagsAndFinishes(t *testing.T) {
	tp := &fakeTraceProvider{}
	h := TraceWith(tp)(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return nil, nil
	})

	_, err := h(&Context{
		Cmd:    123,
		Method: "S.M",
		TraceTags: map[string]string{
			"a": "1",
		},
		Session: Session{Transport: TransportHTTP},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tp.finish != 1 {
		t.Fatalf("expected finish called once, got %d", tp.finish)
	}
	if tp.gotTags["method"] != "S.M" || tp.gotTags["a"] != "1" || tp.gotTags["cmd"] == "" {
		t.Fatalf("unexpected tags: %#v", tp.gotTags)
	}
}

func TestTraceWith_UsesGlobalProviderWhenNil(t *testing.T) {
	tp := &fakeTraceProvider{}
	SetGlobalTraceProvider(tp)
	defer SetGlobalTraceProvider(nil)

	h := TraceWith(nil)(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return nil, nil
	})

	_, err := h(&Context{Cmd: 9, Method: "S.Global", Session: Session{Transport: TransportHTTP}}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tp.finish != 1 {
		t.Fatalf("expected finish called once, got %d", tp.finish)
	}
	if tp.gotTags["method"] != "S.Global" || tp.gotTags["transport"] != string(TransportHTTP) {
		t.Fatalf("unexpected tags: %#v", tp.gotTags)
	}
}

func TestContextInfof_IncludesTraceIDPrefix(t *testing.T) {
	baseCtx := contextWithTraceValues(context.Background(), strings.Repeat("a", 32), strings.Repeat("b", 16), "")
	ic := &captureLogIContext{}
	ctx := WrapIContextWithContext(baseCtx, ic, 1)

	ctx.Infof("hello")
	if !strings.Contains(ic.lastInfo, "trace_id:") {
		t.Fatalf("expected trace prefix in log format, got %q", ic.lastInfo)
	}
	if !strings.Contains(ic.lastInfo, strings.Repeat("a", 32)) {
		t.Fatalf("expected trace id in log format, got %q", ic.lastInfo)
	}
}

func TestHTTPTraceHelpers_RoundTripTraceparent(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/trace", nil)
	if err != nil {
		t.Fatalf("http.NewRequest err = %v", err)
	}
	req.Header.Set("Traceparent", "00-11111111111111111111111111111111-2222222222222222-01")
	base := ExtractHTTPTraceContext(req)
	ctx := WrapIContextWithContext(base, &fakeIContext{}, 1)
	finish := StartTrace(ctx, &minimalTraceProvider{serviceName: "test", exporter: "stdout", samplerRatio: 1}, map[string]string{"span.name": "test.http"})
	if finish != nil {
		finish(nil)
	}
	headers := http.Header{}
	WriteHTTPTraceResponse(ctx, headers)
	if headers.Get("X-Trace-Id") != "11111111111111111111111111111111" {
		t.Fatalf("unexpected trace id header: %q", headers.Get("X-Trace-Id"))
	}
	if headers.Get("Traceparent") == "" {
		t.Fatalf("expected traceparent header to be written")
	}
}

func TestExtractGRPCTraceContext_FromMetadata(t *testing.T) {
	md := metadata.New(map[string]string{
		"traceparent": "00-33333333333333333333333333333333-4444444444444444-01",
	})
	base := ExtractGRPCTraceContext(metadata.NewIncomingContext(context.Background(), md))
	if got := traceIDFromContext(base); got != "33333333333333333333333333333333" {
		t.Fatalf("unexpected trace id from grpc metadata: %q", got)
	}
	if got := spanIDFromContext(base); got != "4444444444444444" {
		t.Fatalf("unexpected span id from grpc metadata: %q", got)
	}
}

func TestAuthWith_RequiredButMissingAuthenticator(t *testing.T) {
	h := AuthWith(nil)(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return nil, nil
	})
	_, err := h(&Context{AuthRequired: true}, nil)
	if ToErrorCode(err) != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected ERR_INTERNAL, got %v err=%v", ToErrorCode(err), err)
	}
}

func TestSignWith_RequiredButMissingVerifier(t *testing.T) {
	h := SignWith(nil)(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return nil, nil
	})
	_, err := h(&Context{SignRequired: true}, nil)
	if ToErrorCode(err) != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected ERR_INTERNAL, got %v err=%v", ToErrorCode(err), err)
	}
}

func TestUIDLock_UsesContextLocker(t *testing.T) {
	locker := &fakeLocker{}
	ctx := &Context{IContext: &fakeIContext{uid: 42}, UIDLocker: locker}
	h := UIDLock()(func(ctx *Context, req proto.Message) (proto.Message, error) { return nil, nil })
	_, err := h(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if locker.lockCnt != 1 || locker.unlockCnt != 1 {
		t.Fatalf("expected lock/unlock once, got lock=%d unlock=%d", locker.lockCnt, locker.unlockCnt)
	}
}

func TestMCPGuardWith_BlocksCall(t *testing.T) {
	m := &fakeMCP{}
	ctx := &Context{MCP: m}

	h := MCPGuardWith(func(ctx *Context, tool string, input any) error {
		return E(g1_protocol.ErrorCode_ERR_INTERNAL, "blocked")
	})(func(ctx *Context, req proto.Message) (proto.Message, error) {
		_, err := ctx.MCP.CallTool("x", nil)
		return nil, err
	})

	_, err := h(ctx, nil)
	if ToErrorCode(err) != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("expected blocked ERR_INTERNAL, got %v err=%v", ToErrorCode(err), err)
	}
	if m.called != 0 {
		t.Fatalf("expected inner MCP not called, got %d", m.called)
	}
}

func TestEffectiveMethodTimeout(t *testing.T) {
	if got := effectiveMethodTimeout(250 * time.Millisecond); got != 250*time.Millisecond {
		t.Fatalf("expected explicit timeout preserved, got %v", got)
	}
	if got := effectiveMethodTimeout(0); got != DefaultMethodTimeout {
		t.Fatalf("expected zero timeout to use default %v, got %v", DefaultMethodTimeout, got)
	}
}

func TestWrapWS_AppliesDefaultTimeoutWhenUnset(t *testing.T) {
	var remaining time.Duration
	h := WrapWS(MethodDesc{Cmd: 1, Name: "test.ws.default-timeout"}, nil,
		func() any { return &fakePB{} },
		func(ctx *Context, req any) (any, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline to be set")
			}
			remaining = time.Until(deadline)
			return nil, nil
		},
	)

	code := h(&fakeIContext{uid: 99}, nil)
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
	if remaining <= 0 || remaining > DefaultMethodTimeout || remaining < 4*time.Second {
		t.Fatalf("expected remaining deadline near default timeout, got %v", remaining)
	}
}

func TestWrapGRPCUnary_AppliesDefaultTimeoutWhenUnset(t *testing.T) {
	var remaining time.Duration
	h := WrapGRPCUnary(MethodDesc{Cmd: 2, Name: "test.grpc.default-timeout"}, nil,
		func(ctx *Context, req any) (any, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("expected deadline to be set")
			}
			remaining = time.Until(deadline)
			return nil, nil
		},
	)

	if _, err := h(context.Background(), &fakePB{}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if remaining <= 0 || remaining > DefaultMethodTimeout || remaining < 4*time.Second {
		t.Fatalf("expected remaining deadline near default timeout, got %v", remaining)
	}
}
