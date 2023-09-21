package net_mgr

import (
	"GoOne/lib/net/kcp_server"
	"GoOne/lib/net/tcp_server"
	"net"
	"sync"

	Kcp "github.com/xtaci/kcp-go/v5"
)

// 必须实现 tcpserver.ITcpPacketSvrEventHandler
type ConnTcpSvr struct {
	tcp_server.TcpPacketSvr

	uidConnMap        map[uint64]net.Conn
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn net.Conn, data []byte)
}

type ConnKcpSvr struct {
	kcp_server.KcpSvr

	uidConnMap        map[uint64]*Kcp.UDPSession
	connUidMap        map[*Kcp.UDPSession]uint64
	remoteAddrConnMap map[string]*Kcp.UDPSession
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn *Kcp.UDPSession, data []byte)
}

func NewTcpSvr() *ConnTcpSvr {
	return &ConnTcpSvr{}
}

func NewKcpSvr() *ConnKcpSvr {
	return &ConnKcpSvr{}
}
