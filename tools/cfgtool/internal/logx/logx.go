package logx

import (
	"fmt"
	"time"
)

// 简单控制台日志，按处理类型区分标识图标
func ts() string {
	return time.Now().Format(time.StampMilli)
}

// 信息（文件列表、上传明细等）
func Infof(format string, args ...interface{}) {
	fmt.Printf("[%s] ℹ️ %s\n", ts(), fmt.Sprintf(format, args...))
}

// 加载（外部proto等资源装载）
func Loadf(format string, args ...interface{}) {
	fmt.Printf("[%s] 📥 %s\n", ts(), fmt.Sprintf(format, args...))
}

// 解析（xlsx/sheet）
func Parsef(format string, args ...interface{}) {
	fmt.Printf("[%s] 🔍 %s\n", ts(), fmt.Sprintf(format, args...))
}

// 注册（自动注册配置）
func Registerf(format string, args ...interface{}) {
	fmt.Printf("[%s] 📋 %s\n", ts(), fmt.Sprintf(format, args...))
}

// 索引（自动主键索引）
func Indexf(format string, args ...interface{}) {
	fmt.Printf("[%s] 🔑 %s\n", ts(), fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	fmt.Printf("[%s] ⚠️ %s\n", ts(), fmt.Sprintf(format, args...))
}

// 生成完成（数据/代码产出）
func Successf(format string, args ...interface{}) {
	fmt.Printf("[%s] ✅ %s\n", ts(), fmt.Sprintf(format, args...))
}

// 最终提示（整体流程收尾）
func Finalf(format string, args ...interface{}) {
	fmt.Printf("[%s] 🏁 %s\n", ts(), fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...interface{}) {
	fmt.Printf("[%s] ❌ %s\n", ts(), fmt.Sprintf(format, args...))
}
