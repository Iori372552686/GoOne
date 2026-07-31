package gamedata

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	contribconfig "github.com/Iori372552686/GoOne/lib/contrib/config"
)

// fakeConfigClient 是 contribconfig.Client 的桩，用于验证 gamedata 远端加载/热更/
// 监听回收（V4 P0-06），不依赖真实配置中心。
type fakeConfigClient struct {
	mu sync.Mutex

	loadErr  error
	watchErr error
	kvs      []*contribconfig.KeyValue

	watchCalled bool
	closed      bool

	watcher *fakeWatcher
}

type fakeWatcher struct {
	mu       sync.Mutex
	ch       chan []*contribconfig.KeyValue
	stopped  bool
	stopChan chan struct{}
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{ch: make(chan []*contribconfig.KeyValue, 8), stopChan: make(chan struct{})}
}

func (w *fakeWatcher) Next() ([]*contribconfig.KeyValue, error) {
	select {
	case kvs := <-w.ch:
		return kvs, nil
	case <-w.stopChan:
		return nil, context.Canceled
	}
}

func (w *fakeWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped {
		w.stopped = true
		close(w.stopChan)
	}
	return nil
}

func (w *fakeWatcher) push(kvs []*contribconfig.KeyValue) {
	w.ch <- kvs
}

func (w *fakeWatcher) isStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func (f *fakeConfigClient) Load() ([]*contribconfig.KeyValue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return f.kvs, nil
}

func (f *fakeConfigClient) Watch() (contribconfig.Watcher, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchCalled = true
	if f.watchErr != nil {
		return nil, f.watchErr
	}
	f.watcher = newFakeWatcher()
	return f.watcher, nil
}

func (f *fakeConfigClient) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

var _ contribconfig.Client = (*fakeConfigClient)(nil)

func kv(key, val string) *contribconfig.KeyValue {
	return &contribconfig.KeyValue{Key: key, Value: []byte(val)}
}

// resetGamedataForTest 清空全局注册表并回收远端状态，使测试互不干扰。
func resetGamedataForTest(t *testing.T) {
	t.Helper()
	StopNet()
	fileMgr = make(map[string]func(string) error)
	t.Cleanup(StopNet)
}

// sheetHolder 以原子方式记录最近一次成功解析的内容，避免热更 goroutine 写、
// 测试 goroutine 读之间的数据竞争。
type sheetHolder struct {
	v atomic.Value
}

func (h *sheetHolder) set(s string) { h.v.Store(s) }

func (h *sheetHolder) get() string {
	if s, ok := h.v.Load().(string); ok {
		return s
	}
	return ""
}

// registerFakeSheet 注册一个可控解析的 sheet：data=="BAD" 时返回解析错误，
// 否则把内容记入 holder。
func registerFakeSheet(name string, h *sheetHolder) {
	Register(name, func(data string) error {
		if data == "BAD" {
			return errors.New("parse error")
		}
		h.set(data)
		return nil
	})
}

// TestInitRemoteRejectsNilClient 验证 nil client 立即返回 error（V4 P0-06）。
func TestInitRemoteRejectsNilClient(t *testing.T) {
	resetGamedataForTest(t)
	if err := InitRemote(nil); err == nil {
		t.Fatal("expected error for nil config client")
	}
}

// TestInitRemoteLoadFailure 验证 Load 失败即整体失败，且不启动 watcher。
func TestInitRemoteLoadFailure(t *testing.T) {
	resetGamedataForTest(t)
	var got sheetHolder
	registerFakeSheet("SheetA", &got)

	fake := &fakeConfigClient{loadErr: errors.New("backend down")}
	if err := InitRemote(fake); err == nil {
		t.Fatal("expected InitRemote to fail when Load fails")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.watchCalled {
		t.Fatal("watcher must not be started when initial Load fails")
	}
}

// TestInitRemoteParseFailure 验证初始化严格模式：任一表解析失败即整体失败，
// 且不启动 watcher（V4 P0-06：不留半启动状态）。
func TestInitRemoteParseFailure(t *testing.T) {
	resetGamedataForTest(t)
	var gotA, gotB sheetHolder
	registerFakeSheet("SheetA", &gotA)
	registerFakeSheet("SheetB", &gotB)

	fake := &fakeConfigClient{kvs: []*contribconfig.KeyValue{
		kv("SheetA.conf", "ok-a"),
		kv("SheetB.conf", "BAD"),
	}}
	if err := InitRemote(fake); err == nil {
		t.Fatal("expected InitRemote to fail when a sheet fails to parse at init")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.watchCalled {
		t.Fatal("watcher must not be started when initial parse fails")
	}
}

// TestInitRemoteMissingSheet 验证初始化严格模式：表缺失/为空即失败。
func TestInitRemoteMissingSheet(t *testing.T) {
	resetGamedataForTest(t)
	var got sheetHolder
	registerFakeSheet("SheetA", &got)

	fake := &fakeConfigClient{kvs: []*contribconfig.KeyValue{kv("SheetA.conf", "  ")}}
	if err := InitRemote(fake); err == nil {
		t.Fatal("expected InitRemote to fail when a sheet is empty at init")
	}
}

// TestHotReloadKeepsOldDataOnParseFailure 验证热更宽松模式：单表解析失败仅记日志，
// 该表旧数据保留；后续好的更新仍可生效（V4 P0-06）。
func TestHotReloadKeepsOldDataOnParseFailure(t *testing.T) {
	resetGamedataForTest(t)
	var got sheetHolder
	registerFakeSheet("SheetA", &got)

	fake := &fakeConfigClient{kvs: []*contribconfig.KeyValue{kv("SheetA.conf", "v1")}}
	if err := InitRemote(fake); err != nil {
		t.Fatalf("InitRemote: %v", err)
	}
	if got.get() != "v1" {
		t.Fatalf("initial data=%q want v1", got.get())
	}

	// 推送坏更新：旧数据必须保留。
	fake.watcher.push([]*contribconfig.KeyValue{kv("SheetA.conf", "BAD")})
	// 给热更 goroutine 处理窗口，随后推一个好更新作为"坏更新已被处理"的屏障。
	fake.watcher.push([]*contribconfig.KeyValue{kv("SheetA.conf", "v2")})
	waitFor(t, time.Second, func() bool { return got.get() == "v2" })
}

// TestStopNetStopsWatcherAndClosesClient 验证 StopNet 停止 watcher、等待热更
// goroutine 退出并 Close client（V4 P0-06：监听可回收）。
func TestStopNetStopsWatcherAndClosesClient(t *testing.T) {
	resetGamedataForTest(t)
	var got sheetHolder
	registerFakeSheet("SheetA", &got)

	fake := &fakeConfigClient{kvs: []*contribconfig.KeyValue{kv("SheetA.conf", "v1")}}
	if err := InitRemote(fake); err != nil {
		t.Fatalf("InitRemote: %v", err)
	}

	StopNet()

	if !fake.watcher.isStopped() {
		t.Fatal("StopNet must stop the watcher")
	}
	fake.mu.Lock()
	closed := fake.closed
	fake.mu.Unlock()
	if !closed {
		t.Fatal("StopNet must close the config client")
	}

	// StopNet 后热更 goroutine 已退出：再推送不应改变数据。
	fake.watcher.push([]*contribconfig.KeyValue{kv("SheetA.conf", "v2")})
	time.Sleep(50 * time.Millisecond)
	if got.get() != "v1" {
		t.Fatalf("data must not change after StopNet, got %q", got.get())
	}
}

// TestStopNetIdempotent 验证 StopNet 幂等（V4 P0-06）。
func TestStopNetIdempotent(t *testing.T) {
	resetGamedataForTest(t)
	var got sheetHolder
	registerFakeSheet("SheetA", &got)
	fake := &fakeConfigClient{kvs: []*contribconfig.KeyValue{kv("SheetA.conf", "v1")}}
	if err := InitRemote(fake); err != nil {
		t.Fatalf("InitRemote: %v", err)
	}
	StopNet()
	StopNet() // 幂等：不 panic
}

// TestSheetFilesSorted 验证 SheetFiles 字典序且带 .conf 后缀（factory DataIDs 用）。
func TestSheetFilesSorted(t *testing.T) {
	resetGamedataForTest(t)
	var s sheetHolder
	registerFakeSheet("SheetB", &s)
	registerFakeSheet("SheetA", &s)
	registerFakeSheet("SheetC", &s)

	files := SheetFiles()
	want := []string{"SheetA.conf", "SheetB.conf", "SheetC.conf"}
	if len(files) != len(want) {
		t.Fatalf("SheetFiles=%v want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("SheetFiles=%v want %v", files, want)
		}
	}
}

// waitFor 在超时内轮询 cond，失败则 fatal。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
