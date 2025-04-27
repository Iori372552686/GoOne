package net_mgr

import (
	"github.com/Iori372552686/GoOne/lib/net/kcp_server"
	"github.com/Iori372552686/GoOne/lib/net/tcp_server"
	"github.com/Iori372552686/GoOne/lib/net/ws_server"
	"net"
	"sync"

	Kcp "github.com/xtaci/kcp-go/v5"
)

type Client struct {
	Uid        uint64
	Zone       uint32
	Conn       net.Conn
	Ip         uint32
	Port       uint32
	RemoteAddr string
}

// 必须实现 tcpserver.ITcpPacketSvrEventHandler
type ConnTcpSvr struct {
	tcp_server.TcpPacketSvr

	uidConnMap        map[uint64]*Client
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

type ConnWsTcpSvr struct {
	ws_server.WsTcpSvr

	uidConnMap        map[uint64]*Client
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
	handler           func(conn net.Conn, data []byte)
}

func NewTcpSvr() *ConnTcpSvr {
	return &ConnTcpSvr{}
}

func NewKcpSvr() *ConnKcpSvr {
	return &ConnKcpSvr{}
}

func NewWsTcpSvr() *ConnWsTcpSvr {
	return &ConnWsTcpSvr{}
}
