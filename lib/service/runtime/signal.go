package runtime

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

// signalSource 把 os.Signal 投递适配为 Run 所需的两个 channel：
//   - termCh 接收第一个终止信号（SIGINT/SIGTERM）。
//   - secondCh 在收到第二次终止信号时关闭，使 Run 能取消进行中的 Drain 并强制
//     Stop。
//
// reload 处理是平台相关的（见 signal_unix.go / signal_windows.go），通过
// installSignals 返回的 reloadCh 暴露。
type signalSource struct {
	termCh   <-chan os.Signal
	secondCh <-chan struct{}
	// reloadCh 在没有重载信号的平台可为 nil。
	reloadCh <-chan os.Signal
	// escalate 在进程内关闭 secondCh。它同时被真实第二信号观察者与测试使用，使
	// 排空取消确定性强，且不依赖向测试二进制投递 OS 信号。
	escalate func()
	stop     func()
}

// installSignals 为平台合适的信号接线 os.Notify，并返回一个 signalSource 以及一
// 个必须在 Run 返回时调用的清理函数（它调用 signal.Stop 并关闭内部第二信号观察
// goroutine）。
//
// 内部 goroutine 观察第二次终止信号；第一个信号直接投递到 termCh。这使 channel
// 有界，并让第二次 SIGINT/SIGTERM 能升级关停。
func installSignals() signalSource {
	termSignals, reloadSignals := platformSignals()

	term := make(chan os.Signal, 1)
	reload := make(chan os.Signal, 1)
	notifySignals(term, termSignals)
	if len(reloadSignals) > 0 {
		notifySignals(reload, reloadSignals)
	}

	// 升级观察者：第二次终止信号关闭 secondDone。secondDone channel 及其关闭由
	// secondOnce 守护，使 OS 信号路径与任何进程内升级都幂等。
	second := make(chan os.Signal, 1)
	notifySignals(second, termSignals)
	secondDone := make(chan struct{})
	var secondOnce sync.Once
	escalate := func() {
		secondOnce.Do(func() { close(secondDone) })
	}
	secondStop := make(chan struct{})
	go func() {
		select {
		case <-second:
			escalate()
		case <-secondStop:
		}
	}()

	return signalSource{
		termCh:   term,
		secondCh: secondDone,
		reloadCh: reloadOrNil(reload, reloadSignals),
		escalate: escalate,
		stop: func() {
			signal.Stop(term)
			signal.Stop(reload)
			signal.Stop(second)
			close(secondStop)
		},
	}
}

// reloadOrNil 在平台有重载信号时返回 reload，否则返回 nil，使 Run 的 select 永不
// 在其上触发。
func reloadOrNil(reload chan os.Signal, reloadSignals []os.Signal) <-chan os.Signal {
	if len(reloadSignals) == 0 {
		return nil
	}
	return reload
}

// notifySignals 是一个小包装，仅当至少有一个信号要观察时才调用 signal.Notify
// （Windows 没有 SIGUSR1）。
func notifySignals(ch chan<- os.Signal, sigs []os.Signal) {
	if len(sigs) == 0 {
		return
	}
	signal.Notify(ch, sigs...)
}

// awaitRunReason 阻塞直到 app 应离开 Ready 阶段。它返回以下之一：
//   - "ctx_done"：父 context 被取消，
//   - "terminated"：第一个终止信号到达，
//   - "reload"：重载信号到达（调用方可重新进入 Ready）。
//
// 在 "reload" 时，调用方处理重载并再次调用本函数继续等待。其他两个原因下调用方
// 进入排空/关停。
func awaitRunReason(ctx context.Context, src signalSource) (string, error) {
	reload := src.reloadCh
	for {
		select {
		case <-ctx.Done():
			return "ctx_done", ctx.Err()
		case <-src.termCh:
			return "terminated", nil
		case sig, ok := <-reload:
			if !ok {
				reload = nil
				continue
			}
			if isReloadSignal(sig) {
				return "reload", nil
			}
			// 到达 reload channel 的非重载信号，仅在平台归类为重载时才按重载处理。
			return "reload", nil
		}
	}
}
