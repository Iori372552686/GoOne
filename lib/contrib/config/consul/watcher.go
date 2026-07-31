package consul

import (
	"github.com/Iori372552686/GoOne/lib/contrib/config"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

// P1-07：后台 watch-plan 错误路由到 errCh，由 Next() 上报给调用方，禁止库 goroutine panic。
type watcher struct {
	source    *source
	ch        chan interface{}
	errCh     chan error // 容量 1：watch-plan 运行期错误在订阅前发生也不丢失
	closeChan chan struct{}
	wp        *watch.Plan
}

func (w *watcher) handle(idx uint64, data interface{}) {
	if data == nil {
		return
	}

	_, ok := data.(api.KVPairs)
	if !ok {
		return
	}

	select {
	case w.ch <- struct{}{}:
	default:
		// 已有待处理的变更通知，丢弃重复唤醒（Next 会重新 Load 全量）。
	}
}

func newWatcher(s *source) (*watcher, error) {
	w := &watcher{
		source:    s,
		ch:        make(chan interface{}, 1),
		errCh:     make(chan error, 1),
		closeChan: make(chan struct{}),
	}

	wp, err := watch.Parse(map[string]interface{}{"type": "keyprefix", "prefix": s.options.path})
	if err != nil {
		return nil, err
	}

	wp.Handler = w.handle
	w.wp = wp

	// wp.Run 是阻塞调用；运行期错误投递到 errCh 由 Next() 上报，不再 panic
	//（P1-07：外部服务异常不得杀死进程）。
	go func() {
		if runErr := wp.RunWithClientAndHclog(s.client, nil); runErr != nil {
			select {
			case w.errCh <- runErr:
			default:
			}
		}
	}()

	return w, nil
}

func (w *watcher) Next() ([]*config.KeyValue, error) {
	select {
	case _, ok := <-w.ch:
		if !ok {
			return nil, nil
		}
		return w.source.Load()
	case err := <-w.errCh:
		// P1-07：watch-plan 运行期失败上报调用方，可触发上层重连/降级。
		return nil, err
	case <-w.closeChan:
		return nil, nil
	}
}

func (w *watcher) Stop() error {
	w.wp.Stop()
	close(w.closeChan)
	return nil
}
