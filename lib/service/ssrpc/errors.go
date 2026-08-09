package ssrpc

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/gerr"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// 注册/分发层的哨兵错误。调用方可用 errors.Is 区分；它们不会被不透明包装。
var (
	// ErrDispatcherSealed 由 Register*E 返回（并由遗留的无返回值 Register* 方法记
	// 录日志），在对已 Seal 的 Dispatcher 尝试注册时触发。
	ErrDispatcherSealed = errors.New("ssrpc: dispatcher 已 seal")
	// ErrNilDispatcher 在注册目标 Dispatcher 为 nil 时返回。
	ErrNilDispatcher = errors.New("ssrpc: nil dispatcher")
	// ErrNilRegistry 在 Registry 方法（Register/Seal）的接收者为 nil 时返回。
	ErrNilRegistry = errors.New("ssrpc: nil registry")
	// ErrNilHandler 在注册 nil handler 时返回。
	ErrNilHandler = errors.New("ssrpc: nil handler")
	// ErrRegistrySealed 在 Seal 后对 Registry 尝试变更时返回。
	ErrRegistrySealed = errors.New("ssrpc: registry 已 seal")
	// ErrDuplicateBinding 在某 Binding key 与已注册项冲突时返回（同批次内或与既有
	// 状态冲突）。
	ErrDuplicateBinding = errors.New("ssrpc: 重复的 binding")
	// ErrEmptyService 在 Register 时服务名为空时返回。
	ErrEmptyService = errors.New("ssrpc: 服务名为空")
	// ErrInvalidBinding 在 Binding 畸形（缺 key、kind 错误等）时返回。
	ErrInvalidBinding = errors.New("ssrpc: 非法 binding")
)

// Error is the canonical error type for GoOne RPC handlers.
//
// Generated wrappers will map returned error to g1_protocol.ErrorCode via ToErrorCode().
type Error struct {
	Code g1_protocol.ErrorCode
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil && e.Msg != "" {
		return fmt.Sprintf("%s | %v", e.Msg, e.Err)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Msg != "" {
		return e.Msg
	}
	return e.Code.String()
}

func (e *Error) Unwrap() error { return e.Err }

func E(code g1_protocol.ErrorCode, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

func Wrap(code g1_protocol.ErrorCode, msg string, err error) *Error {
	return &Error{Code: code, Msg: msg, Err: err}
}

// Unimplemented is a helper for generated scaffold implementations.
// It maps to ERR_INTERNAL by default (you can adjust mapping later if needed).
func Unimplemented(method string) *Error {
	method = strings.TrimSpace(method)
	if method == "" {
		return Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "unimplemented", nil)
	}
	return Wrap(g1_protocol.ErrorCode_ERR_INTERNAL, "unimplemented: "+method, nil)
}

func ToErrorCode(err error) g1_protocol.ErrorCode {
	if err == nil {
		return g1_protocol.ErrorCode_ERR_OK
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Code != 0 {
			return e.Code
		}
	}
	// Framework errors (gerr) carry their own wire code; this also handles
	// plain errors by mapping them to ERR_INTERNAL.
	return gerr.Code(err)
}

// ApplyErrCode 把错误码写入响应消息的业务码字段（reflect，ExtractErrCode 的逆操作）。
// 支持 GoOne 响应消息的三种常见形态：
//  1. 嵌套 Ret *Ret{Code,...} —— 最常见，自动创建 Ret 并设 Code
//  2. 平铺 ErrorCode int32
//  3. 消息本身即 Ret{Code,...}
//
// 用于 WrapUnary：handler 返回 (rsp, err) 时把 err 的码写进 rsp 再回包，
// 使业务可统一 return rsp, gerr.New(...) 而无需手动写 rsp.Ret.Code。
// 无法识别的形态静默跳过（不影响回包本身）。
func ApplyErrCode(rsp any, code g1_protocol.ErrorCode) {
	if rsp == nil {
		return
	}
	applyErrCodeReflect(rsp, code)
}

// applyErrCodeReflect 用 reflect 把 code 写入 rsp 的业务码字段。
func applyErrCodeReflect(rsp any, code g1_protocol.ErrorCode) {
	v := reflect.ValueOf(rsp)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}

	// 形态 1：嵌套 Ret *Ret{Code,...}
	if f := v.FieldByName("Ret"); f.IsValid() && f.Kind() == reflect.Ptr {
		if f.IsNil() {
			// 自动创建 Ret 实例
			newRet := reflect.New(f.Type().Elem())
			f.Set(newRet)
		}
		inner := f.Elem()
		if setInt32Field(inner, "Code", int32(code)) {
			return
		}
	}

	// 形态 2：平铺 ErrorCode int32
	if setInt32Field(v, "ErrorCode", int32(code)) {
		return
	}

	// 形态 3：消息本身是 Ret{Code,Msg}
	setInt32Field(v, "Code", int32(code))
}

// setInt32Field 在 struct v 上找到名为 name 的 int32 字段并赋值；成功返回 true。
func setInt32Field(v reflect.Value, name string, val int32) bool {
	f := v.FieldByName(name)
	if !f.IsValid() || !f.CanSet() {
		return false
	}
	if f.Kind() == reflect.Int32 {
		f.SetInt(int64(val))
		return true
	}
	return false
}


