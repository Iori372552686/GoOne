package ssrpc

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCError converts a g1_protocol.ErrorCode to a gRPC status error.
// ERR_OK / ERR_SUCESS returns nil (no error).
func ToGRPCError(code g1_protocol.ErrorCode) error {
	gc := errorCodeToGRPCCode(code)
	if gc == codes.OK {
		return nil
	}
	return status.Errorf(gc, "%s", code.String())
}

// FromGRPCError extracts a g1_protocol.ErrorCode from a gRPC status error.
// A nil error returns ERR_OK.
func FromGRPCError(err error) g1_protocol.ErrorCode {
	if err == nil {
		return g1_protocol.ErrorCode_ERR_OK
	}
	st, ok := status.FromError(err)
	if !ok {
		return g1_protocol.ErrorCode_ERR_INTERNAL
	}
	return grpcCodeToErrorCode(st.Code())
}

func errorCodeToGRPCCode(code g1_protocol.ErrorCode) codes.Code {
	switch code {
	case g1_protocol.ErrorCode_ERR_SUCESS, g1_protocol.ErrorCode_ERR_OK:
		return codes.OK
	case g1_protocol.ErrorCode_ERR_TIMEOUT:
		return codes.DeadlineExceeded
	case g1_protocol.ErrorCode_ERR_MARSHAL, g1_protocol.ErrorCode_ERR_ARGV:
		return codes.InvalidArgument
	case g1_protocol.ErrorCode_ERR_NOT_EXIST:
		return codes.NotFound
	case g1_protocol.ErrorCode_ERR_HAS_EXIST:
		return codes.AlreadyExists
	case g1_protocol.ErrorCode_ERR_FAIL, g1_protocol.ErrorCode_ERR_INTERNAL:
		return codes.Internal
	case g1_protocol.ErrorCode_ERR_DB:
		return codes.Internal
	case g1_protocol.ErrorCode_ERR_CONF:
		return codes.Internal
	case g1_protocol.ErrorCode_ERR_UNAUTHENTICATED:
		return codes.Unauthenticated
	case g1_protocol.ErrorCode_ERR_UNIMPLEMENTED:
		return codes.Unimplemented
	default:
		return codes.Internal
	}
}

func grpcCodeToErrorCode(c codes.Code) g1_protocol.ErrorCode {
	switch c {
	case codes.OK:
		return g1_protocol.ErrorCode_ERR_OK
	case codes.InvalidArgument:
		return g1_protocol.ErrorCode_ERR_ARGV
	case codes.NotFound:
		return g1_protocol.ErrorCode_ERR_NOT_EXIST
	case codes.AlreadyExists:
		return g1_protocol.ErrorCode_ERR_HAS_EXIST
	case codes.DeadlineExceeded:
		return g1_protocol.ErrorCode_ERR_TIMEOUT
	case codes.Internal:
		return g1_protocol.ErrorCode_ERR_INTERNAL
	case codes.Unavailable:
		return g1_protocol.ErrorCode_ERR_FAIL
	case codes.Unauthenticated:
		return g1_protocol.ErrorCode_ERR_UNAUTHENTICATED
	case codes.Unimplemented:
		return g1_protocol.ErrorCode_ERR_UNIMPLEMENTED
	default:
		return g1_protocol.ErrorCode_ERR_INTERNAL
	}
}
