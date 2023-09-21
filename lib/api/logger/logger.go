package logger

import (
	"fmt"

	"github.com/golang/glog"
)

func Flush() {
	glog.Flush()
}

func Init() {
}

func Fatalf(format string, args ...interface{}) {
	//glog.Fatalf(format, args...)
	glog.FatalDepth(1, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
	glog.ErrorDepth(1, fmt.Sprintf(format, args...))
}

func Warningf(format string, args ...interface{}) {
	glog.WarningDepth(1, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	glog.InfoDepth(1, fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	glog.V(5).Info(fmt.Sprintf(format, args...))
}

func DebugDepthf(depth int, format string, args ...interface{}) {
	glog.V(5).Info(1+depth, fmt.Sprintf(format, args...))
}
