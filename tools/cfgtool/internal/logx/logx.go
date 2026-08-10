package logx

import (
	"fmt"
	"time"
)

// 简单控制台日志，带标识
func ts() string {
	return time.Now().Format(time.StampMilli)
}

func Infof(format string, args ...interface{}) {
	fmt.Printf("[%s] ℹ️ %s\n", ts(), fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	fmt.Printf("[%s] ⚠️ %s\n", ts(), fmt.Sprintf(format, args...))
}

func Successf(format string, args ...interface{}) {
	fmt.Printf("[%s] ✅ %s\n", ts(), fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
	fmt.Printf("[%s] ❌ %s\n", ts(), fmt.Sprintf(format, args...))
}
