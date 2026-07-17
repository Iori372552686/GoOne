//go:build windows

package runtime

import (
	"os"
	"syscall"
)

// platformSignals 返回 Windows 的终止与重载信号集合。Windows 没有
// SIGUSR1/USR2，故此处无重载信号；重载在本平台为 no-op（reload channel 保持
// nil）。
func platformSignals() (term, reload []os.Signal) {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		nil
}

// isReloadSignal 在 Windows 上永远返回 false，因为没有重载信号被观察。
func isReloadSignal(s os.Signal) bool {
	return false
}
