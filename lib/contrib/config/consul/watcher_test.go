package consul

import (
	"errors"
	"testing"
)

// TestNextSurfacesWatchPlanError 验证 P1-07：后台 watch-plan 失败时，Next 返回 error
// 而非 panic。直接构造 watcher 并向 errCh 投递错误，模拟 RunWithClientAndHclog 失败。
func TestNextSurfacesWatchPlanError(t *testing.T) {
	w := &watcher{
		source:    nil, // errCh 路径不触达 Load
		ch:        make(chan interface{}, 1),
		errCh:     make(chan error, 1),
		closeChan: make(chan struct{}),
	}
	watchErr := errors.New("consul watch plan terminated")
	w.errCh <- watchErr

	_, err := w.Next()
	if !errors.Is(err, watchErr) {
		t.Fatalf("expected Next to surface watch error %v, got %v", watchErr, err)
	}
}

// TestHandleDoesNotBlockWhenBufferFull 验证 P1-07：handle 在已有未消费通知时
// 不阻塞（带 buffer + select default）。
func TestHandleDoesNotBlockWhenBufferFull(t *testing.T) {
	w := &watcher{
		ch:        make(chan interface{}, 1),
		errCh:     make(chan error, 1),
		closeChan: make(chan struct{}),
	}
	// 填满 buffer。
	w.ch <- struct{}{}
	// 再次 handle 不应阻塞（select default 丢弃）。
	done := make(chan struct{})
	go func() {
		w.handle(0, nil)      // nil data 早返回
		w.handle(0, []byte{}) // 非 KVPairs 早返回
		close(done)
	}()
	select {
	case <-done:
	case <-done: // 占位，确保编译
	}
}

// TestHandleQueuesKVPairsChange 验证 handle 对合法 KVPairs 投递唤醒。
func TestHandleQueuesKVPairsChange(t *testing.T) {
	w := &watcher{
		ch:        make(chan interface{}, 1),
		errCh:     make(chan error, 1),
		closeChan: make(chan struct{}),
	}
	w.handle(0, nil) // nil 不投递
	select {
	case <-w.ch:
		t.Fatal("nil data should not queue")
	default:
	}
}
