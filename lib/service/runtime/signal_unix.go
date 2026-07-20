//go:build !windows

package runtime

import (
	"os"
	"syscall"
)

// platformSignals 返回 Unix 的终止与重载信号集合。SIGINT 与 SIGTERM 触发关停；
// SIGUSR1 触发配置重载。
func platformSignals() (term, reload []os.Signal) {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		[]os.Signal{syscall.SIGUSR1}
}

// isReloadSignal 上报 s 是否为平台重载信号。
func isReloadSignal(s os.Signal) bool {
	return s == syscall.SIGUSR1
}
