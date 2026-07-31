package net_mgr

import (
	"net"
	"sync"
)

// connLease 跟踪已被 admission 接受（admitted）的底层连接，使 OnClose 只为已 admitted
// 的连接配对释放计数与名额。
//
// 历史缺陷：被 Admission 拒绝的连接不增加计数，但 OnClose 可能被无条件调用并减少计数，
// 造成计数漂移与负数。connLease 用一个以 net.Conn 为键的集合记录"已 admitted"，OnClose
// 仅在集合中存在时才 Release，并在集合中删除（幂等）。
//
// 刻意保持最小：不引入通用资源容器抽象，仅供 TCP/WS/KCP 三传输的 OnConn/OnClose 复用。
type connLease struct {
	mu       sync.Mutex
	admitted map[net.Conn]struct{}
}

func newConnLease() *connLease {
	return &connLease{admitted: make(map[net.Conn]struct{})}
}

// markAdmitted 记录一个连接已被 admission 接受。
func (l *connLease) markAdmitted(conn net.Conn) {
	l.mu.Lock()
	l.admitted[conn] = struct{}{}
	l.mu.Unlock()
}

// takeIfAdmitted 若连接此前被 markAdmitted，从集合移除并返回 true（调用方据此 Release 计数）；
// 否则返回 false（被拒绝的连接或重复 OnClose，不 Release）。
func (l *connLease) takeIfAdmitted(conn net.Conn) bool {
	l.mu.Lock()
	_, ok := l.admitted[conn]
	if ok {
		delete(l.admitted, conn)
	}
	l.mu.Unlock()
	return ok
}

// size 返回当前已 admitted 且未关闭的连接数（测试与指标观测用）。
func (l *connLease) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.admitted)
}

// ensureLease 用于三种传输的 OnConn：若 lease 因兼容路径（如 &ConnTcpSvr{} + SetHub）
// 未初始化，则懒创建，保证 OnConn/OnClose 不会因 nil lease panic。
func (t *ConnTcpSvr) ensureLease() {
	if t.lease == nil {
		t.lease = newConnLease()
	}
}

func (t *ConnWsTcpSvr) ensureLease() {
	if t.lease == nil {
		t.lease = newConnLease()
	}
}

func (t *ConnKcpSvr) ensureLease() {
	if t.lease == nil {
		t.lease = newConnLease()
	}
}

// admissionOf 安全返回 hub 的 AdmissionController（hub 或 Admission 为 nil 时返回 nil）。
// 抽出以减少 OnConn/OnClose 中的重复 nil 检查。
func admissionOf(hub *SessionHub) *AdmissionController {
	if hub == nil {
		return nil
	}
	return hub.Admission()
}
