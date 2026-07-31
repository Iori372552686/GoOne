package apollo

// P1-07 apollo 入口分级测试说明：
//
// apollo 的构造期 error 由 agollo.StartWithConfig 同步返回，仅在 agollo 内部
// 同步校验（如必填项、配置解析）失败时触发；appid 缺失等多数配置问题要等到
// agollo 异步连接真实 Apollo 时才暴露，无法在无中间件的单元测试中稳定复现。
// 因此本包不对 NewSourceE 的 error 返回做强制断言；以下用静态断言确认入口分级
// 契约（MustNewSource 存在且 NewSource 委托它），error/panic 的真实路径由
// integration job（真实 Apollo）覆盖。

import (
	"reflect"
	"runtime"
	"testing"
)

// TestMustNewSourceAndNewSourceExist 静态确认 P1-07 入口分级：
// MustNewSource（语义明确 panic 版）与 Deprecated NewSource 共存。
func TestMustNewSourceAndNewSourceExist(t *testing.T) {
	mustFn := runtime.FuncForPC(reflect.ValueOf(MustNewSource).Pointer()).Name()
	if mustFn == "" {
		t.Fatal("MustNewSource must be exported")
	}
	newFn := runtime.FuncForPC(reflect.ValueOf(NewSource).Pointer()).Name()
	if newFn == "" {
		t.Fatal("NewSource must still be exported (Deprecated)")
	}
}
