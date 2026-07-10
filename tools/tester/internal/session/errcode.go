package session

import (
	"reflect"

	"github.com/golang/protobuf/proto"
)

// ExtractErrCode 从响应消息中提取业务错误码（按协议规范约定的常见形态）：
//  1. 字段 Ret *Ret{Code,Msg} —— 取 Code
//  2. 字段 ErrorCode int32
//  3. 消息本身即 Ret{Code,Msg} —— 取 Code
//
// 无法识别时返回 0（视为成功）。
func ExtractErrCode(m proto.Message) int32 {
	if m == nil {
		return 0
	}
	v := reflect.ValueOf(m)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return 0
	}

	// 形态 1：嵌套 Ret
	if f := v.FieldByName("Ret"); f.IsValid() && f.Kind() == reflect.Ptr && !f.IsNil() {
		inner := f.Elem()
		if inner.Kind() == reflect.Struct {
			if code := inner.FieldByName("Code"); code.IsValid() && code.Kind() == reflect.Int32 {
				return int32(code.Int())
			}
		}
	}

	// 形态 2：平铺 ErrorCode
	if f := v.FieldByName("ErrorCode"); f.IsValid() && f.Kind() == reflect.Int32 {
		return int32(f.Int())
	}

	// 形态 3：消息本身是 Ret
	if code := v.FieldByName("Code"); code.IsValid() && code.Kind() == reflect.Int32 {
		if msg := v.FieldByName("Msg"); msg.IsValid() && msg.Kind() == reflect.String {
			return int32(code.Int())
		}
	}

	return 0
}
