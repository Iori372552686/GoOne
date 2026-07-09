package net_mgr

import (
	"net"
	"sync"

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

	uidConnMap        map[uint64]*Client
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn net.Conn, data []byte)
}

type ConnWsTcpSvr struct {
	ws_server.WsTcpSvr

	uidConnMap        map[uint64]*Client
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn net.Conn, data []byte)
}

// ConnKcpSvr is the KCP gateway session layer; the underlying
// *kcp.UDPSession is handled through net.Conn, so the session model is
// identical to the TCP gateway.
type ConnKcpSvr struct {
	kcp_server.KcpPacketSvr

	uidConnMap        map[uint64]*Client
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn net.Conn, data []byte)
}

func NewTcpSvr() *ConnTcpSvr {
	svr := &ConnTcpSvr{}
	registerGatewaySource("tcp", svr)
	return svr
}

func NewWsTcpSvr() *ConnWsTcpSvr {
	svr := &ConnWsTcpSvr{}
	registerGatewaySource("ws", svr)
	return svr
}

func NewKcpSvr() *ConnKcpSvr {
	svr := &ConnKcpSvr{}
	registerGatewaySource("kcp", svr)
	return svr
}
