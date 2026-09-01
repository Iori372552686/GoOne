package ssrpc

import (
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/gerr"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// TestApplyErrCodeNestedRet 验证把错误码写入嵌套 Ret 字段（最常见形态）。
func TestApplyErrCodeNestedRet(t *testing.T) {
	rsp := &g1_protocol.LoginRsp{}
	ApplyErrCode(rsp, g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER)
	if rsp.Ret == nil {
		t.Fatal("Ret should be auto-created")
	}
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER {
		t.Fatalf("Ret.Code = %v, want %v", rsp.Ret.Code, g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER)
	}
}

// TestApplyErrCodeOverwritesExisting 验证覆盖已有 Ret.Code。
func TestApplyErrCodeOverwritesExisting(t *testing.T) {
	rsp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	ApplyErrCode(rsp, g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH)
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH {
		t.Fatalf("Ret.Code = %v, want ERR_DIAMOND_NOT_ENOUGH", rsp.Ret.Code)
	}
}

// TestApplyErrCodeNilSafe 验证 nil 安全。
func TestApplyErrCodeNilSafe(t *testing.T) {
	ApplyErrCode(nil, g1_protocol.ErrorCode_ERR_FAIL) // should not panic
}

// TestToErrorCodeGerrChain 验证 gerr 错误链的码提取。
func TestToErrorCodeGerrChain(t *testing.T) {
	err := gerr.New(g1_protocol.ErrorCode_ERR_TIMEOUT, "timeout", "db query")
	if code := ToErrorCode(err); code != g1_protocol.ErrorCode_ERR_TIMEOUT {
		t.Fatalf("ToErrorCode(gerr) = %v, want ERR_TIMEOUT", code)
	}

	wrapped := gerr.ErrTimeout.WithMessage("wrapped")
	if code := ToErrorCode(wrapped); code != g1_protocol.ErrorCode_ERR_TIMEOUT {
		t.Fatalf("ToErrorCode(gerr wrapped) = %v, want ERR_TIMEOUT", code)
	}
}

// TestToErrorCodeSsrpcError 验证 ssrpc.Error 的码提取。
func TestToErrorCodeSsrpcError(t *testing.T) {
	err := E(g1_protocol.ErrorCode_ERR_ARGV, "bad argv")
	if code := ToErrorCode(err); code != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("ToErrorCode(ssrpc.E) = %v, want ERR_ARGV", code)
	}
}

// TestToErrorCodePlainError 验证普通 error 映射到 ERR_INTERNAL。
func TestToErrorCodePlainError(t *testing.T) {
	if code := ToErrorCode(errors.New("boom")); code != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("ToErrorCode(plain) = %v, want ERR_INTERNAL", code)
	}
}

func TestUnimplementedUsesDedicatedCode(t *testing.T) {
	err := Unimplemented("AccountService.FutureMethod")
	if code := ToErrorCode(err); code != g1_protocol.ErrorCode_ERR_UNIMPLEMENTED {
		t.Fatalf("Unimplemented code = %v, want ERR_UNIMPLEMENTED", code)
	}
}
