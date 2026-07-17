package runtime

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// SessionTracker 跟踪网关的活跃连接数与活跃会话数，并在两者归零时通知等待者。
//
// 设计要点（遵循 roadmap P0-07）：
//   - ActiveConnections：底层连接已建立且未关闭。
//   - ActiveSessions：已绑定 UID 的逻辑会话；重复绑定不重复计数，OnClose 只减一次。
//   - 等待归零用状态变更通知（broadcast），不用轮询。
//   - 计数用 atomic，永不取负（减到 0 以下记日志并钳为 0）。
//
// 它是 runtime 层的通用原语；P0-08 迁移时由三传输（tcp/ws/kcp）的会话层在 OnConn/
// OnClose/绑定 UID 处调用 Inc/Dec。
type SessionTracker struct {
	connections atomic.Int64
	sessions    atomic.Int64

	mu       sync.Mutex
	cond     *sync.Cond
	closed   bool
}

// NewSessionTracker 构建一个计数归零的 tracker。
func NewSessionTracker() *SessionTracker {
	t := &SessionTracker{}
	t.cond = sync.NewCond(&t.mu)
	return t
}

// IncConnection 在底层连接建立时调用。
func (t *SessionTracker) IncConnection() {
	if t == nil {
		return
	}
	t.connections.Add(1)
}

// DecConnection 在底层连接关闭时调用。重复关闭不会使计数取负（钳为 0 并记日志）。
func (t *SessionTracker) DecConnection() {
	if t == nil {
		return
	}
	v := t.connections.Add(-1)
	if v < 0 {
		t.connections.Store(0)
		logger.Warningf("session_tracker: connection count went negative; clamped to 0")
	}
	t.notifyIfEmpty()
}

// IncSession 在客户端绑定 UID（登录成功）时调用。重复绑定同一会话不重复计数——调用
// 方需保证一次会话生命周期内只 Inc 一次（绑定发生在 OnConn 之后、且未已绑定）。
func (t *SessionTracker) IncSession() {
	if t == nil {
		return
	}
	t.sessions.Add(1)
}

// DecSession 在会话结束（连接关闭或登出解绑）时调用。幂等保护由调用方负责；此处仅保
// 证计数不取负。
func (t *SessionTracker) DecSession() {
	if t == nil {
		return
	}
	v := t.sessions.Add(-1)
	if v < 0 {
		t.sessions.Store(0)
		logger.Warningf("session_tracker: session count went negative; clamped to 0")
	}
	t.notifyIfEmpty()
}

// ActiveConnections 返回当前活跃连接数。
func (t *SessionTracker) ActiveConnections() int64 {
	if t == nil {
		return 0
	}
	return t.connections.Load()
}

// ActiveSessions 返回当前活跃会话数。
func (t *SessionTracker) ActiveSessions() int64 {
	if t == nil {
		return 0
	}
	return t.sessions.Load()
}

// WaitIdle 阻塞直到连接与 session 均归零，或 ctx 超时/取消。用状态变更通知（cond）
// 驱动，不轮询。返回 nil 表示已归零；返回 ctx.Err() 表示超时/取消。
//
// 注意：sync.Cond 无法直接被 context 取消，故 WaitIdle 启动一个后台 goroutine 在
// ctx.Done 时 broadcast 唤醒，并在归零时也唤醒。该 goroutine 在 WaitIdle 返回前一定
// 退出（通过 stop channel），不泄漏。
func (t *SessionTracker) WaitIdle(ctx context.Context) error {
	if t == nil {
		return nil
	}
	// 快速路径：已归零。
	if t.ActiveConnections() == 0 && t.ActiveSessions() == 0 {
		return nil
	}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			t.mu.Lock()
			t.cond.Broadcast() // ctx 超时，唤醒等待者
			t.mu.Unlock()
		case <-stop:
		}
	}()

	t.mu.Lock()
	defer t.mu.Unlock()
	for !t.closed &&
		(t.connections.Load() > 0 || t.sessions.Load() > 0) {
		// 在 ctx 仍有效期间等待。
		if err := ctx.Err(); err != nil {
			return err
		}
		t.cond.Wait()
		// 被唤醒后循环再检查；若因 ctx.Done 唤醒，下一轮 ctx.Err() 非 nil 即返回。
	}
	if t.closed {
		return nil
	}
	return ctx.Err()
}

// Close 释放 tracker，唤醒所有 WaitIdle 等待者。之后 Inc/Dec 仍可安全调用（计数继
// 续），但 WaitIdle 立即返回 nil。
func (t *SessionTracker) Close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.closed = true
	t.cond.Broadcast()
	t.mu.Unlock()
}

// notifyIfEmpty 在计数归零时 broadcast，唤醒 WaitIdle 等待者。调用方负责在 Dec 之后
// 调用。
func (t *SessionTracker) notifyIfEmpty() {
	if t.ActiveConnections() == 0 && t.ActiveSessions() == 0 {
		t.mu.Lock()
		t.cond.Broadcast()
		t.mu.Unlock()
	}
}
