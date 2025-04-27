package tos

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"unsafe"
)

var savedOSArgs []string

func init() {
	argLen := len(os.Args)
	savedOSArgs = make([]string, len(os.Args))
	for i := 0; i < argLen; i++ {
		savedOSArgs[i] = string([]byte(os.Args[i]))
	}
}

//GetSavedOSArgs 获取保存下来的系统参数
func GetSavedOSArgs() (args []string) {
	argLen := len(savedOSArgs)
	args = make([]string, argLen)
	for i := 0; i < argLen; i++ {
		args[i] = savedOSArgs[i]
	}
	return
}

//SetProcName 设置进程名称(完整的进程名，包含进程的可执行路径，所有的参数以及额外自定义参数，需要使用者手动拼接os.Args)
func SetProcName(name string) {
	argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
	argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:]
	copy(argv0, name)
	argv0[len(name)] = 0
}

//SetProcNameEx 设置进程名（只需要设置自己需要的参数即可，例如进程启动的时候设置的-c/-v 等参数不需要设置）
func SetProcNameEx(name string) {
	argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
	argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:]
	argList := append(savedOSArgs, name)
	argStr := strings.Join(argList, " ")
	copy(argv0, argStr)
	argv0[len(argStr)] = 0
}

// ProcessName return process name
func ProcessName() string {
	segs := strings.Split(savedOSArgs[0], string(os.PathSeparator))
	var name string
	switch runtime.GOOS {
	case "windows":
		name = strings.Split(segs[len(segs)-1], ".")[0]
	case "linux":
		name = segs[len(segs)-1]
	}
	return name
}
