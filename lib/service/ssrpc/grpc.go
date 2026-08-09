package ssrpc

import (
	"context"
	"errors"
	"strconv"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/metadata"
)

// ---------------------------------------------------------------------------
// grpcIContext -- implements cmd_handler.IContext for gRPC transport.
// ---------------------------------------------------------------------------

// grpcIContext extracts identity from gRPC metadata headers (x-uid, x-zone).
// Since gRPC responses are returned directly via the handler return value,
// SendMsgBack is a no-op (same approach as the HTTP transport).
type grpcIContext struct {
	ctx  context.Context
	uid  uint64
	zone uint32
	rid  uint64
}

var _ cmd_handler.IContext = (*grpcIContext)(nil)

// newGRPCIContext creates a grpcIContext, extracting uid/zone from incoming
// gRPC metadata if present.
func newGRPCIContext(ctx context.Context) *grpcIContext {
	var uid uint64
	var zone uint32
	var rid uint64
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-uid"); len(vals) > 0 {
			uid, _ = strconv.ParseUint(vals[0], 10, 64)
		}
		if vals := md.Get("x-zone"); len(vals) > 0 {
			v, _ := strconv.ParseUint(vals[0], 10, 32)
			zone = uint32(v)
		}
		if vals := md.Get("x-rid"); len(vals) > 0 {
			rid, _ = strconv.ParseUint(vals[0], 10, 64)
		}
	}
	return &grpcIContext{ctx: ctx, uid: uid, zone: zone, rid: rid}
}

func (g *grpcIContext) Uid() uint64         { return g.uid }
func (g *grpcIContext) Zone() uint32        { return g.zone }
func (g *grpcIContext) Rid() uint64         { return g.rid }
func (g *grpcIContext) OriSrcBusId() uint32 { return 0 }
func (g *grpcIContext) Ip() uint32          { return 0 }
func (g *grpcIContext) Flag() uint32        { return 0 }

func (g *grpcIContext) ParseMsg(data []byte, msg proto.Message) error {
	return proto.Unmarshal(data, msg)
}

func (g *grpcIContext) SendMsgBack(proto.Message) {} // no-op: gRPC returns directly

func (g *grpcIContext) CallMsgBySvrType(uint32, g1_protocol.CMD, proto.Message, proto.Message) error {
	return errGRPCUnsupported
}
func (g *grpcIContext) CallMsgByRouter(uint32, uint64, g1_protocol.CMD, proto.Message, proto.Message) error {
	return errGRPCUnsupported
}
func (g *grpcIContext) CallOtherMsgBySvrType(uint32, uint64, uint64, uint32, g1_protocol.CMD, proto.Message, proto.Message) error {
	return errGRPCUnsupported
}
func (g *grpcIContext) SendMsgByServerType(uint32, g1_protocol.CMD, proto.Message) error {
	return errGRPCUnsupported
}
func (g *grpcIContext) SendMsgByRouter(uint32, uint64, g1_protocol.CMD, proto.Message) error {
	return errGRPCUnsupported
}

var errGRPCUnsupported = errors.New("operation not supported in grpc context")

func (g *grpcIContext) Errorf(format string, args ...interface{})   { logger.Errorf(format, args...) }
func (g *grpcIContext) Warningf(format string, args ...interface{}) { logger.Warningf(format, args...) }
func (g *grpcIContext) Infof(format string, args ...interface{})    { logger.Infof(format, args...) }
func (g *grpcIContext) Debugf(format string, args ...interface{})   { logger.Debugf(format, args...) }

// Context returns the underlying context.Context.
func (g *grpcIContext) Context() context.Context { return g.ctx }

// ---------------------------------------------------------------------------
// GRPCUnaryHandler + WrapGRPCUnary
// ---------------------------------------------------------------------------

// GRPCUnaryHandler handles a single gRPC unary request.
type GRPCUnaryHandler func(ctx context.Context, req any) (any, error)

// WrapGRPCUnary returns a GRPCUnaryHandler for the gRPC transport.
//
// Unlike WrapUnary/WrapWS (which receive raw bytes), gRPC gives us an
// already-decoded proto message, so WrapGRPCUnary skips ParseMsg.
func WrapGRPCUnary(desc MethodDesc, mws []Middleware, invoke func(ctx *Context, req any) (any, error)) GRPCUnaryHandler {
	mws = prepareMW(mws, desc.UIDLock)
	h := buildHandler(mws, invoke) // pre-build chain once at init time
	return func(grpcCtx context.Context, reqAny any) (any, error) {
		ic := newGRPCIContext(grpcCtx)
		ctx := WrapIContext(ic, desc.Cmd)
		ctx.SetTransport(TransportGRPC)
		applyDesc(ctx, &desc)
		ctx.ApplyTimeout(effectiveMethodTimeout(desc.Timeout))
		defer ctx.Close()

		req, ok := reqAny.(proto.Message)
		if !ok || req == nil {
			return nil, ToGRPCError(g1_protocol.ErrorCode_ERR_INTERNAL)
		}

		rsp, err := h(ctx, req)
		if err != nil {
			return nil, ToGRPCError(ToErrorCode(err))
		}
		if desc.OneWay {
			return nil, nil
		}
		return rsp, nil
	}
}
