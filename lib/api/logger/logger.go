package logger

import (
	"fmt"
	"reflect"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger/plug"
	"github.com/Iori372552686/GoOne/lib/api/logger/zap"
)

var logPlug *plug.CmdBlacklist

func Flush() {
	//logger.Flush()
	zap.Sync()
}

func init() {
	logPlug = plug.NewCmdBlacklist()
	logPlug.Register(131076) //CMD_MAIN_HEARTBEAT_REQ
	logPlug.Register(131077) //CMD_MAIN_HEARTBEAT_RSP
}

// init logger
func InitLogger(logPath string, level string, name string) (zap.Logger, error) {
	if logPath == "" {
		logPath = "./logs"
	}
	if level == "" {
		level = "debug"
	}
	if name == "" {
		name = "log_" + datetime.GetDataHMS()
	}

	return zap.InitLogger(zap.Config{
		Level:       level,
		LogFileName: fmt.Sprintf("%s.log", name),
		LogDir:      logPath,
		LogStdout:   true,
	})
}

// reg cmd to black list
func RegisterCmdBacklist(cmds ...uint32) {
	for _, cmd := range cmds {
		logPlug.Register(cmd)
	}
}

func Fatalf(format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	plug.UploadFatalToDingHook(text)
	zap.Errorf(text)
}

func FatalfDepthf(depth int, format string, args ...interface{}) {
	zap.Errorf(format, args...)
}

func ErrorDepthf(depth int, format string, args ...interface{}) {
	zap.Errorf(format, args...)
}

func ErrorDepth(depth int, format string, args ...interface{}) {
	zap.Errorf(format, args...)
}

func WarningDepthf(depth int, format string, args ...interface{}) {
	zap.Warnf(format, args...)
}

func WarningDepth(depth int, format string, args ...interface{}) {
	zap.Warnf(format, args...)
}

func InfoDepthf(depth int, format string, args ...interface{}) {
	zap.Infof(format, args...)
}

func InfoDepth(depth int, format string, args ...interface{}) {
	zap.Infof(format, args...)
}

func DebugDepthf(depth int, format string, args ...interface{}) {
	zap.Debugf(format, args...)
}

func CmdDebugDepthf(cmd uint32, depth int, format string, args ...interface{}) {
	if !logPlug.IsBlocked(cmd) {
		//logger.V(5).InfoDepth(1+depth, fmt.Sprintf(format, args...))
		zap.Debugf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	zap.Errorf(format, args...)
}

func Error(args ...interface{}) {
	zap.Errorf(defaultFormat(args))
}

func Warningf(format string, args ...interface{}) {
	zap.Warnf(format, args...)
}

func Infof(format string, args ...interface{}) {
	zap.Infof(format, args...)
}

func Debugf(format string, args ...interface{}) {
	zap.Debugf(format, args...)
}

func CmdDebugf(cmd uint32, format string, args ...interface{}) {
	if !logPlug.IsBlocked(cmd) {
		zap.Debugf(format, args...)
		//DebugDepthf(1, fmt.Sprintf(format, args...))
	}
}

func defaultFormat(args []any) string {
	n := len(args)
	switch n {
	case 0:
		return ""
	case 1:
		return "%v"
	}

	b := make([]byte, 0, n*3-1)
	wasString := true // Suppress leading space.
	for _, arg := range args {
		isString := arg != nil && reflect.TypeOf(arg).Kind() == reflect.String
		if wasString || isString {
			b = append(b, "%v"...)
		} else {
			b = append(b, " %v"...)
		}
		wasString = isString
	}
	return string(b)
}
