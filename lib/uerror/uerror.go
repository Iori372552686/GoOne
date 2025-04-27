package uerror

import (
	"fmt"
	"runtime"
)

type UError struct {
	file  string // 文件名
	line  int    // 文件行号
	fname string // 函数名
	code  int32  // 错误码
	msg   string // 错误
}

func (d *UError) Code() int32 {
	return d.code
}

func (d *UError) Msg() string {
	return d.msg
}

func (d *UError) Error() string {
	return fmt.Sprintf("%s:%d\n%s\t%d: %s", d.file, d.line, d.fname, d.code, d.msg)
}

func New(skip int, code int32, format string, msgs ...interface{}) *UError {
	// 获取调用堆栈
	pc, file, line, _ := runtime.Caller(skip)
	funcName := runtime.FuncForPC(pc).Name()
	// 返回错误
	return &UError{
		file:  file,
		line:  line,
		fname: funcName,
		code:  code,
		msg:   fmt.Sprintf(format, msgs...),
	}
}

func GetCodeMsg(err error) (code int32, errmsg string) {
	switch vv := err.(type) {
	case *UError:
		code, errmsg = vv.Code(), vv.Error()
	case error:
		code, errmsg = -1, err.Error()
	}
	return
}
