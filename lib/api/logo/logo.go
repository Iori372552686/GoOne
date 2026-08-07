// Package logo 在服务启动时打印框架 CLI 标识与版本信息。
//
// 设计上保持零第三方依赖、纯 fmt 输出
// 避免日志重定向到文件时出现 ANSI 控制字符乱码。version 为手写常量，
// 发版时手动维护；GoVersion 动态取自 runtime.Version()。
package logo

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Version 是 GoOne 框架版本，发版时手动维护。
const Version = "v1.2.7"

const logoArt = ` 
 ██████╗  ██████╗  ██████╗ ███╗   ██╗███████╗
██╔════╝ ██╔═══██╗██╔═══██╗████╗  ██║██╔════╝
██║  ███╗██║   ██║██║   ██║██╔██╗ ██║█████╗
██║   ██║██║   ██║██║   ██║██║╚██╗██║██╔══╝
╚██████╔╝╚██████╔╝╚██████╔╝██║ ╚████║███████╗
 ╚═════╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚══════╝
`

// boxWidth 是信息框的内部内容宽度（即上下边框中 ─ 的数量），内容行据此对齐。
const boxWidth = 40

// 信息框使用的 Unicode 制表符。
const (
	cornerTL = "┌"
	cornerTR = "┐"
	cornerBL = "└"
	cornerBR = "┘"
	pipe     = "│"
	dash     = "─"
)

// Print 打印框架 logo 与版本信息框。name 通常为服务名（如 connsvr）。
func Print(name string) {
	fmt.Println(strings.TrimSuffix(strings.TrimPrefix(logoArt, "\n"), "\n"))
	printBox(
		fmt.Sprintf("Service   : %s", name),
		fmt.Sprintf("Version   : %s", Version),
		fmt.Sprintf("GoVersion : %s", runtime.Version()),
	)
}

// printBox 用 Unicode 制表符画一个固定宽度的信息框，每行内容左对齐并右侧补空格。
// 内容行的填充宽度与上下边框的 ─ 数量相同（boxWidth），左右各加一个 │，
// 因此整行显示宽度 = boxWidth + 2，与边框 ┌─…─┐ 严格对齐。
func printBox(rows ...string) {
	border := cornerTL + strings.Repeat(dash, boxWidth) + cornerTR
	fmt.Println(border)
	for _, row := range rows {
		inner := " " + row
		pad := boxWidth - utf8.RuneCountInString(inner)
		if pad > 0 {
			inner += strings.Repeat(" ", pad)
		}
		fmt.Println(pipe + inner + pipe)
	}
	border = cornerBL + strings.Repeat(dash, boxWidth) + cornerBR
	fmt.Println(border)
}
