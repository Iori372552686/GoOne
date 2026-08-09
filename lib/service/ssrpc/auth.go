package ssrpc

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// Authenticator performs authentication for a request.
// It should return nil when the request is authenticated, otherwise an error.
type Authenticator interface {
	Authenticate(ctx *Context, req proto.Message) error
}

// SignVerifier verifies request signature for a request.
type SignVerifier interface {
	Verify(ctx *Context, req proto.Message) error
}

// AuthWith enforces AuthRequired via the provided Authenticator.
// If AuthRequired=true and authenticator is nil, it returns ERR_INTERNAL.
func AuthWith(a Authenticator) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if ctx == nil || !ctx.AuthRequired {
				return next(ctx, req)
			}
			if a == nil {
				return nil, Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "authenticator not configured", nil)
			}
			if err := a.Authenticate(ctx, req); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

// SignWith enforces SignRequired via the provided SignVerifier.
// If SignRequired=true and verifier is nil, it returns ERR_INTERNAL.
func SignWith(v SignVerifier) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if ctx == nil || !ctx.SignRequired {
				return next(ctx, req)
			}
			if v == nil {
				return nil, Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "sign verifier not configured", nil)
			}
			if err := v.Verify(ctx, req); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}


