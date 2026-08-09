package ssrpc

import (
	"context"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// ServerStream is a generic wrapper around grpc.ServerStream that provides
// type-safe Send for server-streaming RPC methods.
type ServerStream[T proto.Message] struct {
	grpc.ServerStream
}

// Send marshals and sends a single message to the client.
func (s *ServerStream[T]) Send(msg T) error {
	return s.ServerStream.SendMsg(msg)
}

// Context returns the server stream's context.
func (s *ServerStream[T]) Context() context.Context {
	return s.ServerStream.Context()
}

// NewServerStream wraps a grpc.ServerStream into a typed ServerStream.
func NewServerStream[T proto.Message](ss grpc.ServerStream) *ServerStream[T] {
	return &ServerStream[T]{ServerStream: ss}
}

// GRPCStreamHandler is the function type returned by WrapGRPCServerStream.
type GRPCStreamHandler func(srv any, stream grpc.ServerStream) error

// WrapGRPCServerStreamTyped adapts a typed business callback to the raw
// grpc.ServerStream-based runtime wrapper.
func WrapGRPCServerStreamTyped[T proto.Message](
	desc MethodDesc,
	mws []Middleware,
	newReq func() any,
	invoke func(ctx *Context, req any, stream *ServerStream[T]) error,
) GRPCStreamHandler {
	return WrapGRPCServerStream(desc, mws, newReq, func(ctx *Context, req any, stream grpc.ServerStream) error {
		return invoke(ctx, req, NewServerStream[T](stream))
	})
}

// WrapGRPCServerStream returns a GRPCStreamHandler for server-streaming RPCs.
//
// The invoke callback receives the ssrpc.Context, the decoded request, and the
// raw grpc.ServerStream. The business layer should wrap the stream into
// ServerStream[T] for type-safe sending.
func WrapGRPCServerStream(
	desc MethodDesc,
	mws []Middleware,
	newReq func() any,
	invoke func(ctx *Context, req any, stream grpc.ServerStream) error,
) GRPCStreamHandler {
	mws = prepareMW(mws, desc.UIDLock)
	return func(_ any, stream grpc.ServerStream) error {
		baseCtx := ExtractGRPCTraceContext(stream.Context())
		ic := newGRPCIContext(baseCtx)
		ctx := WrapIContextWithContext(baseCtx, ic, desc.Cmd)
		ctx.SetTransport(TransportGRPC)
		applyDesc(ctx, &desc)
		ctx.ApplyTimeout(effectiveMethodTimeout(desc.Timeout))
		var traceErr error
		finish := StartTrace(ctx, nil, map[string]string{
			"span.name":  "ssrpc.transport.grpc.stream",
			"span.phase": "transport",
			"span.kind":  "server",
		})
		WriteGRPCStreamTraceResponse(stream, ctx)
		defer func() {
			if finish != nil {
				finish(traceErr)
			}
			ctx.Close()
		}()

		// Decode request from stream.
		reqAny := newReq()
		req, ok := reqAny.(proto.Message)
		if !ok || req == nil {
			traceErr = E(g1_protocol.ErrorCode_ERR_INTERNAL, "invalid req type")
			return ToGRPCError(g1_protocol.ErrorCode_ERR_INTERNAL)
		}
		if err := stream.RecvMsg(req); err != nil {
			traceErr = err
			return err
		}

		// Run middleware chain via a unary wrapper that delegates to streaming invoke.
		var streamErr error
		h := buildHandler(mws, func(c2 *Context, reqIn any) (any, error) {
			msg, _ := reqIn.(proto.Message)
			streamErr = invoke(c2, msg, stream)
			return nil, nil
		})

		_, mwErr := h(ctx, req)
		if mwErr != nil {
			traceErr = mwErr
			return ToGRPCError(ToErrorCode(mwErr))
		}
		traceErr = streamErr
		return streamErr
	}
}
