package tcp_server

import (
	"net"
)

type ITcpSvrEventHandler interface {
	OnConn(net.Conn)             // 被Listener协程调用，一个TcpSvr对应一个Listener协程
	OnRead(net.Conn, []byte) int // 被Read协程调用，每个Connection对应一个Read协调
	OnRead2(net.Conn, []byte) int
	OnClose(net.Conn) // 被Read协程调用，每个Connection对应一个Read协调
}

type ITcpPacketSvrEventHandler interface {
	OnConn(net.Conn)           // 被Listener协程调用，一个TcpPacketSvr对应一个Listener协程
	OnPacket(net.Conn, []byte) // 被Read协程调用，每个Connection对应一个Read协调
	OnClose(net.Conn)          // 被Read协程调用，每个Connection对应一个Read协调
}
