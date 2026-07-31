package net_mgr

import (
	"net"

	"github.com/Iori372552686/GoOne/lib/net/kcp_server"
	"github.com/Iori372552686/GoOne/lib/net/tcp_server"
	"github.com/Iori372552686/GoOne/lib/net/ws_server"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

type Client struct {
	Uid        uint64
	Zone       uint32
	Conn       net.Conn
	Ip         uint32
	Port       uint32
	RemoteAddr string
}

// GatewayServer is the unified session-facing interface every gateway
// transport must satisfy (modelled after due's network layer). New
// transports (e.g. a future KCP gateway) must implement it fully before
// entering the production path.
type GatewayServer interface {
	SendByUid(uid uint64, data1 []byte, data2 []byte) error
	BroadcastByZone(zone int32, data1 []byte, data2 []byte)
	Kick(uid uint64, reason g1_protocol.EKickOutReason)
	KickByRemoteAddr(uid uint64, reason g1_protocol.EKickOutReason, remoteAddr string)
	GetClientByUid(uid uint64) *Client
	UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *Client
}

var (
	_ GatewayServer = (*ConnTcpSvr)(nil)
	_ GatewayServer = (*ConnWsTcpSvr)(nil)
	_ GatewayServer = (*ConnKcpSvr)(nil)
)

// gatewayTransport abstracts the wire backend used by a gateway session
// layer: the goroutine-based gonet stack or the event-driven gnet stack.
type gatewayTransport interface {
	WriteData(conn net.Conn, data1 []byte, data2 []byte) error
	Close(conn net.Conn) error
}

// 必须实现 tcpserver.ITcpPacketSvrEventHandler
type ConnTcpSvr struct {
	tcp_server.TcpPacketSvr

	// transport 指向实际网络后端：gonet 时为嵌入的 TcpPacketSvr，
	// gnet 时为事件驱动 server。所有下行写与关闭必须经 transport。
	transport gatewayTransport

	// hub 是共享会话状态拥有者（P0-05）。必须非 nil（生产由 connsvr globals 注入
	// 单一 SessionHub），使 TCP/WS/KCP 共享会话状态。V3-P1-02：本地 map 路径已删除，
	// 三传输内部始终走 hub。
	hub *SessionHub

	// lease 跟踪已 admitted 连接，使 OnClose 只为 admitted 连接释放计数（V4 P0-04）。
	lease *connLease

	handler func(conn net.Conn, data []byte)
}

type ConnWsTcpSvr struct {
	ws_server.WsTcpSvr

	// hub 同 ConnTcpSvr。必须非 nil（生产由 connsvr globals 注入）。
	hub *SessionHub

	// lease 同 ConnTcpSvr（V4 P0-04）。
	lease *connLease

	handler func(conn net.Conn, data []byte)
}

// ConnKcpSvr is the KCP gateway session layer; the underlying
// *kcp.UDPSession is handled through net.Conn, so the session model is
// identical to the TCP gateway.
type ConnKcpSvr struct {
	kcp_server.KcpPacketSvr

	// hub 同 ConnTcpSvr。必须非 nil（生产由 connsvr globals 注入）。
	hub *SessionHub

	// lease 同 ConnTcpSvr（V4 P0-04）。
	lease *connLease

	handler func(conn net.Conn, data []byte)
}

func NewTcpSvr() *ConnTcpSvr {
	svr := &ConnTcpSvr{hub: NewSessionHub(nil), lease: newConnLease()}
	registerGatewaySource("tcp", svr)
	return svr
}

func NewWsTcpSvr() *ConnWsTcpSvr {
	svr := &ConnWsTcpSvr{hub: NewSessionHub(nil), lease: newConnLease()}
	registerGatewaySource("ws", svr)
	return svr
}

func NewKcpSvr() *ConnKcpSvr {
	svr := &ConnKcpSvr{hub: NewSessionHub(nil), lease: newConnLease()}
	registerGatewaySource("kcp", svr)
	return svr
}

// NewTcpSvrWithHub 构造一个显式注入共享 SessionHub 的 TCP 网关（V4 P0-04）。
// hub 为 nil 时返回 error，保证生产装配不会遗留 nil hub 导致后续 OnConn 跳过 admission。
func NewTcpSvrWithHub(hub *SessionHub) (*ConnTcpSvr, error) {
	if hub == nil {
		return nil, errNilHub
	}
	svr := &ConnTcpSvr{hub: hub, lease: newConnLease()}
	registerGatewaySource("tcp", svr)
	return svr, nil
}

// NewWsTcpSvrWithHub 构造一个显式注入共享 SessionHub 的 WS 网关（V4 P0-04）。
// hub 为 nil 时返回 error。
func NewWsTcpSvrWithHub(hub *SessionHub) (*ConnWsTcpSvr, error) {
	if hub == nil {
		return nil, errNilHub
	}
	svr := &ConnWsTcpSvr{hub: hub, lease: newConnLease()}
	registerGatewaySource("ws", svr)
	return svr, nil
}

// NewKcpSvrWithHub 构造一个显式注入共享 SessionHub 的 KCP 网关（V4 P0-04）。
// hub 为 nil 时返回 error。
func NewKcpSvrWithHub(hub *SessionHub) (*ConnKcpSvr, error) {
	if hub == nil {
		return nil, errNilHub
	}
	svr := &ConnKcpSvr{hub: hub, lease: newConnLease()}
	registerGatewaySource("kcp", svr)
	return svr, nil
}

// errNilHub 在显式构造器收到 nil hub 时返回（V4 P0-04）。
var errNilHub = newHubError("net_mgr: hub must not be nil; use NewSessionHub or inject the shared hub")

type hubError string

func (e hubError) Error() string { return string(e) }

func newHubError(msg string) error { return hubError(msg) }

// SetHub 注入共享 SessionHub（P0-05）。三种传输（TCP/WS/KCP）必须注入同一个 hub 实
// 例，使同一 UID 跨传输重绑原子化。必须在 Start 前调用。
//
// Deprecated: 新代码应在构造时注入 hub（NewTcpSvrWithHub 等）；本方法保留兼容 connsvr
// globals 装配。调用方须保证在 Start 前调用一次；Start 后调用行为未定义。
func (t *ConnTcpSvr) SetHub(h *SessionHub)   { t.hub = h }
func (t *ConnWsTcpSvr) SetHub(h *SessionHub) { t.hub = h }
func (t *ConnKcpSvr) SetHub(h *SessionHub)   { t.hub = h }
