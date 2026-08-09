package ssrpc

import (
	"context"
	"io"
	"net"
	"testing"

	optionsv1 "github.com/Iori372552686/GoOne/api/gen/goone/options/v1"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// ---------------------------------------------------------------------------
// WrapGRPCUnary tests
// ---------------------------------------------------------------------------

func TestWrapGRPCUnary_SetsTransportGRPC(t *testing.T) {
	var gotTransport Transport
	desc := MethodDesc{Cmd: 100, Name: "test.Login"}
	h := WrapGRPCUnary(desc, nil,
		func(ctx *Context, req any) (any, error) {
			gotTransport = ctx.Transport
			return nil, nil
		},
	)

	ctx := context.Background()
	_, err := h(ctx, &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotTransport != TransportGRPC {
		t.Fatalf("expected transport %q, got %q", TransportGRPC, gotTransport)
	}
}

func TestWrapGRPCUnary_ExtractsMetadata(t *testing.T) {
	var gotUid uint64
	var gotZone uint32
	desc := MethodDesc{Cmd: 200, Name: "test.Info"}
	h := WrapGRPCUnary(desc, nil,
		func(ctx *Context, req any) (any, error) {
			gotUid = ctx.Uid()
			gotZone = ctx.Zone()
			return nil, nil
		},
	)

	md := metadata.New(map[string]string{"x-uid": "12345", "x-zone": "7"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := h(ctx, &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotUid != 12345 {
		t.Fatalf("uid = %d, want 12345", gotUid)
	}
	if gotZone != 7 {
		t.Fatalf("zone = %d, want 7", gotZone)
	}
}

func TestWrapGRPCUnary_RunsMiddleware(t *testing.T) {
	var order []string
	mw := func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			order = append(order, "mw")
			rsp, err := next(ctx, req)
			order = append(order, "mw-after")
			return rsp, err
		}
	}

	desc := MethodDesc{Cmd: 300, Name: "test.MW"}
	h := WrapGRPCUnary(desc, []Middleware{mw},
		func(ctx *Context, req any) (any, error) {
			order = append(order, "handler")
			return nil, nil
		},
	)

	_, err := h(context.Background(), &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(order) != 3 || order[0] != "mw" || order[1] != "handler" || order[2] != "mw-after" {
		t.Fatalf("middleware order: %v", order)
	}
}

func TestWrapGRPCUnary_PropagatesMethodDesc(t *testing.T) {
	var gotMethod string
	var gotAuth, gotSign bool
	desc := MethodDesc{Cmd: 400, Name: "test.Desc", Auth: true, Sign: true}
	h := WrapGRPCUnary(desc, nil,
		func(ctx *Context, req any) (any, error) {
			gotMethod = ctx.Method
			gotAuth = ctx.AuthRequired
			gotSign = ctx.SignRequired
			return nil, nil
		},
	)

	h(context.Background(), &fakePB{})
	if gotMethod != "test.Desc" || !gotAuth || !gotSign {
		t.Fatalf("method=%q auth=%v sign=%v", gotMethod, gotAuth, gotSign)
	}
}

func TestWrapGRPCUnary_ErrorMapping(t *testing.T) {
	desc := MethodDesc{Cmd: 500, Name: "test.Err"}
	h := WrapGRPCUnary(desc, nil,
		func(ctx *Context, req any) (any, error) {
			return nil, E(g1_protocol.ErrorCode_ERR_ARGV, "bad param")
		},
	)

	_, err := h(context.Background(), &fakePB{})
	if err == nil {
		t.Fatal("expected error")
	}
	code := FromGRPCError(err)
	if code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("expected ERR_ARGV, got %v", code)
	}
}

func TestWrapGRPCUnary_NilReq_ReturnsError(t *testing.T) {
	desc := MethodDesc{Cmd: 600}
	h := WrapGRPCUnary(desc, nil,
		func(ctx *Context, req any) (any, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	)

	_, err := h(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil req")
	}
}

// ---------------------------------------------------------------------------
// Dispatcher gRPC tests
// ---------------------------------------------------------------------------

func TestDispatcher_RegisterGRPCUnary(t *testing.T) {
	d := NewDispatcher()
	called := false

	d.RegisterGRPCUnary("TestService", "TestMethod", func() any { return new(fakePB) },
		func(ctx context.Context, req any) (any, error) {
			called = true
			return nil, nil
		})

	if len(d.grpcMethods) != 1 {
		t.Fatalf("expected 1 grpc method, got %d", len(d.grpcMethods))
	}
	m := d.grpcMethods[0]
	if m.ServiceName != "TestService" || m.MethodName != "TestMethod" {
		t.Fatalf("unexpected service/method: %s/%s", m.ServiceName, m.MethodName)
	}
	if m.IsServerStream {
		t.Fatal("expected unary, got stream")
	}

	_, err := m.UnaryHandler(context.Background(), &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("handler not called")
	}
}

func TestDispatcher_RegisterGRPCStream(t *testing.T) {
	d := NewDispatcher()
	d.RegisterGRPCStream("TestService", "StreamMethod", func(_ any, _ grpc.ServerStream) error {
		return nil
	})

	if len(d.grpcMethods) != 1 {
		t.Fatalf("expected 1 grpc method, got %d", len(d.grpcMethods))
	}
	m := d.grpcMethods[0]
	if !m.IsServerStream {
		t.Fatal("expected server stream")
	}
	if m.StreamDesc == nil {
		t.Fatal("expected StreamDesc to be non-nil")
	}
}

func TestDispatcher_RegisterGRPCUnary_NilDispatcher(t *testing.T) {
	var d *Dispatcher
	// Should not panic.
	d.RegisterGRPCUnary("Svc", "M", func() any { return new(fakePB) }, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
}

func TestDispatcher_RegisterGRPCUnary_NilHandler(t *testing.T) {
	d := NewDispatcher()
	d.RegisterGRPCUnary("Svc", "M", func() any { return new(fakePB) }, nil)
	if len(d.grpcMethods) != 0 {
		t.Fatal("nil handler should not be registered")
	}
}

func TestDispatcher_MountGRPC_UnaryRoundTrip(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	d := NewDispatcher()
	d.RegisterGRPCUnary("test.grpc.v1.Svc", "Ping", func() any { return new(optionsv1.SsRpc) },
		WrapGRPCUnary(MethodDesc{Name: "test ping"}, nil, func(ctx *Context, req any) (any, error) {
			in := req.(*optionsv1.SsRpc)
			return &optionsv1.SsRpc{Comment: "pong", Cmd: in.GetCmd() + 1}, nil
		}))
	d.MountGRPC(srv)

	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient err: %v", err)
	}
	defer conn.Close()

	var rsp optionsv1.SsRpc
	err = conn.Invoke(ctx, "/test.grpc.v1.Svc/Ping", &optionsv1.SsRpc{Cmd: 7}, &rsp)
	if err != nil {
		t.Fatalf("Invoke err: %v", err)
	}
	if rsp.GetComment() != "pong" || rsp.GetCmd() != 8 {
		t.Fatalf("unexpected unary response: comment=%q cmd=%d", rsp.GetComment(), rsp.GetCmd())
	}
}

func TestDispatcher_MountGRPC_ServerStreamRoundTrip(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	d := NewDispatcher()
	d.RegisterGRPCStream("test.grpc.v1.Svc", "Watch", WrapGRPCServerStreamTyped[*optionsv1.SsRpc](
		MethodDesc{Name: "test watch"},
		nil,
		func() any { return new(optionsv1.SsRpc) },
		func(ctx *Context, req any, stream *ServerStream[*optionsv1.SsRpc]) error {
			in := req.(*optionsv1.SsRpc)
			if err := stream.Send(&optionsv1.SsRpc{Comment: "first", Cmd: in.GetCmd() + 1}); err != nil {
				return err
			}
			return stream.Send(&optionsv1.SsRpc{Comment: "second", Cmd: in.GetCmd() + 2})
		},
	))
	d.MountGRPC(srv)

	go func() {
		_ = srv.Serve(lis)
	}()
	defer srv.Stop()

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient err: %v", err)
	}
	defer conn.Close()

	desc := &grpc.StreamDesc{ServerStreams: true}
	stream, err := conn.NewStream(ctx, desc, "/test.grpc.v1.Svc/Watch")
	if err != nil {
		t.Fatalf("NewStream err: %v", err)
	}
	if err := stream.SendMsg(&optionsv1.SsRpc{Cmd: 7}); err != nil {
		t.Fatalf("SendMsg err: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend err: %v", err)
	}

	var rsp1, rsp2 optionsv1.SsRpc
	if err := stream.RecvMsg(&rsp1); err != nil {
		t.Fatalf("RecvMsg #1 err: %v", err)
	}
	if err := stream.RecvMsg(&rsp2); err != nil {
		t.Fatalf("RecvMsg #2 err: %v", err)
	}
	if rsp1.GetComment() != "first" || rsp1.GetCmd() != 8 || rsp2.GetComment() != "second" || rsp2.GetCmd() != 9 {
		t.Fatalf("unexpected stream responses: %#v / %#v", &rsp1, &rsp2)
	}

	var rsp3 optionsv1.SsRpc
	if err := stream.RecvMsg(&rsp3); err != io.EOF {
		t.Fatalf("expected EOF after server stream, got %v", err)
	}
}
