package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// ErrSessionTrackerClosed 在 Close 后等待者（WaitIdle/WaitSessions）返回，表示计数未归
// 零即被强制关闭。不得返回 nil 冒充成功排空。
var ErrSessionTrackerClosed = errors.New("session tracker closed before drain completed")

// SessionTracker 跟踪网关的活跃连接数与活跃会话数，并在两者归零时通知等待者。
//
// 设计要点：
//   - ActiveConnections：底层连接已建立且未关闭。
//   - ActiveSessions：已绑定 UID 的逻辑会话；重复绑定不重复计数，OnClose 只减一次。
//   - 等待归零用状态变更通知（broadcast），不用轮询。
//   - 计数用 atomic + CAS 防止 underflow（旧实现 Add(-1) 后 Store(0) 是 racy 的覆盖）。
//   - Close 在非零计数时让等待者返回 ErrSessionTrackerClosed，不返回 nil 冒充成功排空。
//
// 它是 runtime 层的通用原语；由三传输（tcp/ws/kcp）的会话层在 OnConn/OnClose/绑定
// UID 处调用 Inc/Dec。
type SessionTracker struct {
	connections atomic.Int64
	sessions    atomic.Int64

	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
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

// DecConnection 在底层连接关闭时调用。用 CAS 循环防止 underflow 覆盖并发 Inc
//（旧实现 Add(-1) 后 Store(0) 会覆盖并发 +1）。
func (t *SessionTracker) DecConnection() {
	if t == nil {
		return
	}
	t.decCAS(&t.connections, "connection")
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

// DecSession 在会话结束（连接关闭或登出解绑）时调用。用 CAS 循环防止 underflow。
// 会话归零时广播，唤醒 WaitSessions（即便 connection 未归零）。
func (t *SessionTracker) DecSession() {
	if t == nil {
		return
	}
	t.decCAS(&t.sessions, "session")
	t.notifyIfEmpty()
	if t.ActiveSessions() == 0 {
		// WaitSessions 只关心会话归零，故会话归零时额外广播（connection 可能仍 >0）。
		t.mu.Lock()
		t.cond.Broadcast()
		t.mu.Unlock()
	}
}

// decCAS 用 CAS 循环把计数减一，若已为 0 则记日志并保持 0，不取负、不覆盖并发 Inc。
func (t *SessionTracker) decCAS(counter *atomic.Int64, kind string) {
	for {
		old := counter.Load()
		if old <= 0 {
			// 已为 0：迟到的 Dec，不取负。
			logger.Warningf("session_tracker: %s count already zero; ignored Dec", kind)
			return
		}
		if counter.CompareAndSwap(old, old-1) {
			return
		}
		// CAS 失败：并发 Inc/Dec 改变了值，重试。
	}
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

// WaitSessions 阻塞直到 ActiveSessions 归零，或 ctx 超时/取消，或 Close 被调用。
// 网关 Drain 用它等待逻辑会话排空。返回 nil 表示会话已归零；ctx.Err 表示
// 超时/取消；ErrSessionTrackerClosed 表示 Close 时仍有未归零会话。
//
// 用状态变更通知驱动，不轮询。
func (t *SessionTracker) WaitSessions(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if t.ActiveSessions() == 0 {
		return nil
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			t.mu.Lock()
			t.cond.Broadcast()
			t.mu.Unlock()
		case <-stop:
		}
	}()

	t.mu.Lock()
	defer t.mu.Unlock()
	for !t.closed && t.sessions.Load() > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		t.cond.Wait()
	}
	if t.closed {
		// Close 时若会话已归零，返回 nil；否则返回 ErrSessionTrackerClosed。
		if t.sessions.Load() == 0 {
			return nil
		}
		return ErrSessionTrackerClosed
	}
	return ctx.Err()
}

// WaitIdle 阻塞直到连接与 session 均归零，或 ctx 超时/取消。用状态变更通知（cond）
// 驱动，不轮询。返回 nil 表示已归零；返回 ctx.Err() 表示超时/取消。
//
// 注意：sync.Cond 无法直接被 context 取消，故 WaitIdle 启动一个后台 goroutine 在
// ctx.Done 时 broadcast 唤醒，并在归零时也唤醒。该 goroutine 在 WaitIdle 返回前一定
// 退出（通过 stop channel），不泄漏。
//
// WaitIdle 保留给完整连接归零检查；网关 Drain 用 WaitSessions 只等逻辑会话。
// Close 后若计数未归零返回 ErrSessionTrackerClosed，不返回 nil 冒充成功排空。
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
		// Close 时若已归零返回 nil；否则 ErrSessionTrackerClosed。
		if t.connections.Load() == 0 && t.sessions.Load() == 0 {
			return nil
		}
		return ErrSessionTrackerClosed
	}
	return ctx.Err()
}

// Close 释放 tracker，唤醒所有 WaitIdle/WaitSessions 等待者。之后 Inc/Dec 仍可安全调
// 用（计数继续）。
//
// Close 后等待者根据计数是否归零返回 nil 或 ErrSessionTrackerClosed，绝不返回
// nil 冒充成功排空（历史实现无论计数都返回 nil）。
func (t *SessionTracker) Close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.closed = true
	t.cond.Broadcast()
	t.mu.Unlock()
}

// notifyIfEmpty 在计数归零时 broadcast，唤醒 WaitIdle/WaitSessions 等待者。调用方负责
// 在 Dec 之后调用。
func (t *SessionTracker) notifyIfEmpty() {
	if t.ActiveConnections() == 0 && t.ActiveSessions() == 0 {
		t.mu.Lock()
		t.cond.Broadcast()
		t.mu.Unlock()
	}
}

