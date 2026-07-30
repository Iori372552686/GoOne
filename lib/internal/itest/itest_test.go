package itest_test

import (
	"testing"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
)

// TestEnabledReflectsEnv 验证 Enabled() 跟随 GOONE_INTEGRATION 环境变量。
// Require 的 Skip 行为由各中间件测试在实际使用中验证（默认开发机无中间件即 Skip）。
func TestEnabledReflectsEnv(t *testing.T) {
	t.Setenv("GOONE_INTEGRATION", "0")
	if itest.Enabled() {
		t.Fatal("expected disabled when GOONE_INTEGRATION=0")
	}

	t.Setenv("GOONE_INTEGRATION", "1")
	if !itest.Enabled() {
		t.Fatal("expected enabled when GOONE_INTEGRATION=1")
	}
}

// TestRequireSkipsWhenDisabled 验证默认（未开启集成）时 Require 触发 Skip。
// 用子测试隔离：若 Require 没跳过，FailNow 会标记失败。
func TestRequireSkipsWhenDisabled(t *testing.T) {
	t.Setenv("GOONE_INTEGRATION", "0")
	// Require 应调用 t.SkipNow；若未跳过则下面的标记不会执行（因为 SkipNow 已 Goexit），
	// 只有在错误地未跳过时才会到达并失败。
	t.Run("group", func(t *testing.T) {
		itest.Require(t, "127.0.0.1:9")
		t.Fatal("Require should have skipped, but continued")
	})
}
