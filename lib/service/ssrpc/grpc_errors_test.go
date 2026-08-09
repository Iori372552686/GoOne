package ssrpc

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToGRPCError_OK_ReturnsNil(t *testing.T) {
	if err := ToGRPCError(g1_protocol.ErrorCode_ERR_OK); err != nil {
		t.Fatalf("expected nil for ERR_OK, got %v", err)
	}
	if err := ToGRPCError(g1_protocol.ErrorCode_ERR_SUCESS); err != nil {
		t.Fatalf("expected nil for ERR_SUCESS, got %v", err)
	}
}

func TestToGRPCError_MapsCorrectly(t *testing.T) {
	tests := []struct {
		code     g1_protocol.ErrorCode
		wantGRPC codes.Code
	}{
		{g1_protocol.ErrorCode_ERR_TIMEOUT, codes.DeadlineExceeded},
		{g1_protocol.ErrorCode_ERR_MARSHAL, codes.InvalidArgument},
		{g1_protocol.ErrorCode_ERR_ARGV, codes.InvalidArgument},
		{g1_protocol.ErrorCode_ERR_NOT_EXIST, codes.NotFound},
		{g1_protocol.ErrorCode_ERR_HAS_EXIST, codes.AlreadyExists},
		{g1_protocol.ErrorCode_ERR_INTERNAL, codes.Internal},
		{g1_protocol.ErrorCode_ERR_FAIL, codes.Internal},
	}

	for _, tt := range tests {
		err := ToGRPCError(tt.code)
		if err == nil {
			t.Fatalf("expected non-nil error for %v", tt.code)
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status for %v", tt.code)
		}
		if st.Code() != tt.wantGRPC {
			t.Fatalf("code %v: got gRPC %v, want %v", tt.code, st.Code(), tt.wantGRPC)
		}
	}
}

func TestFromGRPCError_NilReturnsOK(t *testing.T) {
	code := FromGRPCError(nil)
	if code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("expected ERR_OK, got %v", code)
	}
}

func TestFromGRPCError_MapsCorrectly(t *testing.T) {
	tests := []struct {
		grpcCode codes.Code
		wantCode g1_protocol.ErrorCode
	}{
		{codes.OK, g1_protocol.ErrorCode_ERR_OK},
		{codes.InvalidArgument, g1_protocol.ErrorCode_ERR_ARGV},
		{codes.NotFound, g1_protocol.ErrorCode_ERR_NOT_EXIST},
		{codes.AlreadyExists, g1_protocol.ErrorCode_ERR_HAS_EXIST},
		{codes.DeadlineExceeded, g1_protocol.ErrorCode_ERR_TIMEOUT},
		{codes.Internal, g1_protocol.ErrorCode_ERR_INTERNAL},
		{codes.Unavailable, g1_protocol.ErrorCode_ERR_FAIL},
		{codes.PermissionDenied, g1_protocol.ErrorCode_ERR_INTERNAL}, // unmapped -> INTERNAL
	}

	for _, tt := range tests {
		err := status.Errorf(tt.grpcCode, "test")
		code := FromGRPCError(err)
		if code != tt.wantCode {
			t.Fatalf("grpc %v: got %v, want %v", tt.grpcCode, code, tt.wantCode)
		}
	}
}

func TestRoundTrip_ErrorCode_GRPC_ErrorCode(t *testing.T) {
	// Verify: ErrorCode -> gRPC error -> ErrorCode round-trips for key codes.
	roundTrip := []g1_protocol.ErrorCode{
		g1_protocol.ErrorCode_ERR_TIMEOUT,
		g1_protocol.ErrorCode_ERR_NOT_EXIST,
		g1_protocol.ErrorCode_ERR_HAS_EXIST,
		g1_protocol.ErrorCode_ERR_INTERNAL,
	}

	for _, code := range roundTrip {
		err := ToGRPCError(code)
		got := FromGRPCError(err)
		if got != code {
			t.Fatalf("round-trip failed: %v -> gRPC -> %v", code, got)
		}
	}
}
