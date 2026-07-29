package net_mgr

import (
	"errors"
	"net"
	"sync"

	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/util/convert"
)

// ErrGatewayDraining 在网关排空（Quiesce）期间拒绝新会话绑定时返回。
// pack_proc 据此拒绝首包登录，不向后端转发（P0-05）。
var ErrGatewayDraining = errors.New("gateway is draining")

// ActivityCounter 是 SessionHub 上报给运行期的活跃度计数接口。SessionTracker 实现
// 它；hub 在连接/会话建立与释放时调用对应方法。IncConnection 计底层连接，IncSession
// 计已绑定 UID 的逻辑会话（P0-05/P0-06）。
type ActivityCounter interface {
	IncConnection()
	DecConnection()
	IncSession()
	DecSession()
	ActiveConnections() int64
	ActiveSessions() int64
}

// noopCounter 在未接线 SessionTracker 时避免 nil 检查。
type noopCounter struct{}

func (noopCounter) IncConnection()       {}
func (noopCounter) DecConnection()       {}
func (noopCounter) IncSession()          {}
func (noopCounter) DecSession()          {}
func (noopCounter) ActiveConnections() int64 { return 0 }
func (noopCounter) ActiveSessions() int64    { return 0 }

// SessionHub 是三种传输（TCP/WS/KCP）共享的会话状态拥有者（P0-05）。它消除三份重复
// 的 uidConnMap/connUidMap/remoteAddrConnMap/remoteAddrKickMap，并把"同一 UID 跨传输
// 重绑"做成原子操作。
//
// 关键不变量（遵循方案 B P0-05）：
//   - 所有 map 更新都在 hub 锁内完成。
//   - 网络写、Marshal、Close、Kick 通知都在锁外执行（hub 方法只返回不可变快照/指针，
//     调用方在锁外做 I/O）。
//   - 同一 UID 跨 TCP/WS/KCP 重绑是原子的；新连接替换旧连接时会话总数不增加。
//   - 旧连接的迟到 OnClose 不得删除新连接，也不得重复减少会话计数（由 conn 标识比对
//     保证）。
//   - 未认证连接只计入 connection，不计入 session；Drain 只等待 session。
type SessionHub struct {
	counter ActivityCounter

	mu sync.RWMutex
	// accepting=false 后 BindClient 返回 ErrGatewayDraining（Quiesce）。
	accepting bool

	uidConnMap        map[uint64]*Client
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
}

// NewSessionHub 构建一个共享 hub。counter 为 nil 时用 noopCounter；生产由 connsvr
// globals 注入单一 SessionTracker。
func NewSessionHub(counter ActivityCounter) *SessionHub {
	if counter == nil {
		counter = noopCounter{}
	}
	return &SessionHub{
		counter:           counter,
		accepting:         true,
		uidConnMap:        make(map[uint64]*Client),
		connUidMap:        make(map[net.Conn]uint64),
		remoteAddrConnMap: make(map[string]net.Conn),
		remoteAddrKickMap: make(map[string]bool),
	}
}

// Quiesce 标记 hub 不再接受新会话绑定。既有的未认证连接仍可被处理，但 BindClient 拒
// 绝。幂等。
func (h *SessionHub) Quiesce() {
	h.mu.Lock()
	h.accepting = false
	h.mu.Unlock()
}

// ActiveConnections / ActiveSessions 透传给 counter，供 admin 与 Drain 使用。
func (h *SessionHub) ActiveConnections() int64 { return h.counter.ActiveConnections() }
func (h *SessionHub) ActiveSessions() int64    { return h.counter.ActiveSessions() }

// Accepting 上报 hub 是否仍接受新绑定（Quiesce 后为 false）。
func (h *SessionHub) Accepting() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.accepting
}

// BindClient 把一个连接绑定到 UID（登录成功时调用）。返回新 Client 与被替换的旧
// Client（若有，调用方负责在锁外 kick 旧连接）。
//
// 关键不变量：
//   - Quiesce 后返回 ErrGatewayDraining，不修改任何状态（P0-05：pack_proc 据此拒绝首
//     包登录，不向后端转发）。
//   - 在一把 hub 锁内完成：查旧连接、写新索引（uid→client、conn→uid、remoteAddr→conn）。
//     同一 UID 已有连接时，返回 oldCli 供调用方 kick，但会话计数不重复增加（
//     IncSession 只在 uid 首次出现时调用）。
//   - 地址解析用 net.SplitHostPort（兼容 IPv4/IPv6/带 zone），失败返回 error，不制造
//     错误 IP/port。
func (h *SessionHub) BindClient(conn net.Conn, uid uint64, zone uint32) (*Client, *Client, error) {
	remoteAddr := conn.RemoteAddr().String()
	host, portStr, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil, nil, err
	}

	newIns := &Client{
		Uid:        uid,
		Zone:       zone,
		Conn:       conn,
		RemoteAddr: remoteAddr,
		Ip:         bus.IpStringToInt(host),
		Port:       uint32(convert.StrToInt(portStr)),
	}

	h.mu.Lock()
	if !h.accepting {
		h.mu.Unlock()
		return nil, nil, ErrGatewayDraining
	}

	var oldCli *Client
	isNewUID := false
	if prev, exists := h.uidConnMap[uid]; exists {
		oldCli = prev
	} else {
		isNewUID = true
	}

	h.connUidMap[conn] = uid
	h.uidConnMap[uid] = newIns
	h.remoteAddrConnMap[remoteAddr] = conn
	h.mu.Unlock()

	// 计数在锁外：新增 UID 才计 session（重绑不重复计）。
	if isNewUID {
		h.counter.IncSession()
	}
	return newIns, oldCli, nil
}

// GetClientByUid 返回 UID 对应的 Client 快照指针（Client 建成后不可变，故可直接返回）。
func (h *SessionHub) GetClientByUid(uid uint64) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.uidConnMap[uid]
}

// ClientForSend 返回 UID 对应的不可变 Client 指针，供调用方在锁外写网络。若 UID 不存
// 在返回 nil。
func (h *SessionHub) ClientForSend(uid uint64) *Client {
	return h.GetClientByUid(uid)
}

// SnapshotByZone 返回 zone 内（zone<=0 表示全部）的 Client 快照切片，供调用方在锁外
// 逐个写网络。一个慢连接不得阻塞其它连接的发送（P0-05）。
func (h *SessionHub) SnapshotByZone(zone int32) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Client, 0, len(h.uidConnMap))
	for _, c := range h.uidConnMap {
		if zone > 0 && c.Zone != uint32(zone) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ConnByRemoteAddr 返回 remoteAddr 对应的 conn（用于 KickByRemoteAddr）。标记 kick 后由
// MarkKick 记录，避免被 kick 的连接 OnClose 触发登出包。
func (h *SessionHub) ConnByRemoteAddr(remoteAddr string) net.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.remoteAddrConnMap[remoteAddr]
}

// MarkKick 标记某 remoteAddr 正在被 kick（其 OnClose 不应触发登出包）。
func (h *SessionHub) MarkKick(remoteAddr string) {
	h.mu.Lock()
	h.remoteAddrKickMap[remoteAddr] = true
	h.mu.Unlock()
}

// RemoveConn 在连接关闭时调用。返回 (uid, kicked)：
//   - uid==0：conn 未绑定或已被新连接替换（迟到 OnClose），调用方不做任何事。
//   - uid!=0 && kicked：该连接是因 kick 关闭，调用方不触发登出包。
//   - uid!=0 && !kicked：正常关闭，调用方触发登出包。
//
// 关键不变量（P0-05）：旧连接的迟到 OnClose 不得删除新连接——通过 conn 标识比对保证
//（uidConnMap[uid].Conn != conn 时说明已被替换，返回 uid=0）。DecSession 也只在真正
// 移除当前 conn 时调用一次。
func (h *SessionHub) RemoveConn(conn net.Conn) (uid uint64, kicked bool) {
	remoteAddr := conn.RemoteAddr().String()
	h.mu.Lock()
	uid, exists := h.connUidMap[conn]
	if !exists {
		h.mu.Unlock()
		return 0, false
	}
	delete(h.remoteAddrConnMap, remoteAddr)
	delete(h.connUidMap, conn)
	wasKicked := false
	if cur, ok := h.uidConnMap[uid]; ok && cur.Conn == conn {
		delete(h.uidConnMap, uid)
		if h.remoteAddrKickMap[remoteAddr] {
			delete(h.remoteAddrKickMap, remoteAddr)
			wasKicked = true
		}
	} else {
		// uid 已被新连接替换；迟到的旧 OnClose 不动新连接、不减计数。
		h.mu.Unlock()
		return 0, false
	}
	h.mu.Unlock()
	h.counter.DecSession()
	return uid, wasKicked
}

// IncConnection/DecConnection 透传给 counter，供传输层在 OnConn/OnClose 调用。
func (h *SessionHub) IncConnection() { h.counter.IncConnection() }
func (h *SessionHub) DecConnection() { h.counter.DecConnection() }
