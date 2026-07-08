package gerr

import (
	"errors"
	"fmt"
	"testing"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func TestCodeExtraction(t *testing.T) {
	if got := Code(nil); got != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("Code(nil) = %v, want ERR_OK", got)
	}

	err := New(g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER, "role_not_found", "role %d not loaded", 42)
	if got := Code(err); got != g1_protocol.ErrorCode_ERR_NOT_EXIST_PLAYER {
		t.Fatalf("Code = %v, want ERR_NOT_EXIST_PLAYER", got)
	}

	if got := Code(errors.New("plain")); got != g1_protocol.ErrorCode_ERR_INTERNAL {
		t.Fatalf("Code(plain) = %v, want ERR_INTERNAL", got)
	}
}

func TestWrapPreservesChain(t *testing.T) {
	cause := errors.New("dial refused")
	err := Wrap(g1_protocol.ErrorCode_ERR_DB, "redis_set_failed", cause)

	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause must be matchable with errors.Is")
	}
	if got := Code(err); got != g1_protocol.ErrorCode_ERR_DB {
		t.Fatalf("Code = %v, want ERR_DB", got)
	}
	if got := Reason(err); got != "redis_set_failed" {
		t.Fatalf("Reason = %q", got)
	}
}

func TestCodeThroughFmtWrap(t *testing.T) {
	inner := New(g1_protocol.ErrorCode_ERR_TIMEOUT, "timeout", "rpc timed out")
	outer := fmt.Errorf("call mainsvr: %w", inner)

	if got := Code(outer); got != g1_protocol.ErrorCode_ERR_TIMEOUT {
		t.Fatalf("Code through %%w = %v, want ERR_TIMEOUT", got)
	}
	if !errors.Is(outer, ErrTimeout) {
		t.Fatal("errors.Is(outer, ErrTimeout) must hold")
	}
}

func TestSentinelMatching(t *testing.T) {
	err := ErrTimeout.WithMessage("wait rsp for cmd %d", 123)
	if !errors.Is(err, ErrTimeout) {
		t.Fatal("WithMessage must preserve sentinel identity")
	}
	if errors.Is(err, ErrClosed) {
		t.Fatal("distinct sentinels must not match")
	}
}
