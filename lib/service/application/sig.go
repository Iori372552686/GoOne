//go:build !windows
// +build !windows

package application

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

func SignalNotify() {
	signal.Notify(sig, syscall.SIGABRT, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
}

func (a *Application) checkSysSignal() {
	select {
	case s := <-sig:
		switch s {
		case syscall.SIGUSR1:
			glog.Infoln("onreload")
			a.reload()
		default:
			glog.Infoln("onexit")
			a.exit()
			glog.Flush()
			os.Exit(0)
		}
	default:
	}
}
