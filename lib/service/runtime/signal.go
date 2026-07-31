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

	// injectForTest 把一个信号直接注入 dispatcher 的原始输入 channel，使测试可以
	// 确定性地模拟第 N 次终止信号，而无需依赖 OS 自投递（在 Windows 上不可靠）。
	// 生产路径下不调用；nil 表示未接线（如 reload-only source）。
	injectForTest func(os.Signal)
}

// installSignals 为平台合适的信号接线 os.Notify，并返回一个 signalSource 以及一
// 个必须在 Run 返回时调用的清理函数（它调用 signal.Stop 并关闭内部 dispatcher
// goroutine）。
//
// 关键不变量（修复）：整个进程只对 SIGINT/SIGTERM 调用**一次** signal.Notify，
// 指向单一原始 channel；由单一 dispatcher goroutine 计数并据此分别发送 first/second
// 事件。
//
// 历史缺陷：曾用两个 signal.Notify channel 同时订阅 SIGINT/SIGTERM，os/signal 会把
// 每个信号实例投递给所有已注册 channel，导致第一次信号就同时进入 termCh 与
// secondCh，使 Drain 在开始的同时被 escalation 取消。
func installSignals() signalSource {
	termSignals, reloadSignals := platformSignals()

	raw := make(chan os.Signal, 1)
	reload := make(chan os.Signal, 1)
	notifySignals(raw, termSignals)
	if len(reloadSignals) > 0 {
		notifySignals(reload, reloadSignals)
	}

	termOut := make(chan os.Signal, 1)
	secondDone := make(chan struct{})
	var secondOnce sync.Once
	escalate := func() {
		secondOnce.Do(func() { close(secondDone) })
	}
	dispatcherStop := make(chan struct{})

	// 单一 dispatcher：统计从 raw 收到的终止信号。第一个投递到 termOut；第二个关闭
	// secondDone。非终止信号（理论上不会到达 raw，因为只 Notify 了终止信号集）忽略。
	go func() {
		count := 0
		for {
			select {
			case sig := <-raw:
				if !isTermSignal(sig) {
					continue
				}
				count++
				switch count {
				case 1:
					select {
					case termOut <- sig:
					default:
						// termOut 容量 1；若未被消费（异常），不阻塞 dispatcher。
					}
				default:
					// 第二次及以后的终止信号：升级，关闭 secondDone。
					escalate()
				}
			case <-dispatcherStop:
				return
			}
		}
	}()

	return signalSource{
		termCh:   termOut,
		secondCh: secondDone,
		reloadCh: reloadOrNil(reload, reloadSignals),
		escalate: escalate,
		stop: func() {
			signal.Stop(raw)
			signal.Stop(reload)
			close(dispatcherStop)
		},
		injectForTest: func(sig os.Signal) {
			select {
			case raw <- sig:
			default:
				// raw 容量 1；测试按序注入，正常路径不会到这里。若 dispatcher 已停
				// 止或 channel 满，丢弃而不阻塞测试。
			}
		},
	}
}

// platformTermSignals 仅返回平台终止信号集，供测试注入使用。
func platformTermSignals() []os.Signal {
	term, _ := platformSignals()
	return term
}

// isTermSignal 上报 s 是否属于平台终止信号集。dispatcher 只对终止信号计数；重载信
// 号走独立的 reload channel，不会到达 raw。
func isTermSignal(s os.Signal) bool {
	term, _ := platformSignals()
	for _, t := range term {
		if t == s {
			return true
		}
	}
	return false
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
