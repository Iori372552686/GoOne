package ws_server

import (
	"net"
)

type IWsTcpSvrEventHandler interface {
	OnConn(net.Conn)             // 被Listener协程调用，一个WsSvr对应一个Listener协程
	OnRead(net.Conn, []byte) int // 被Read协程调用，每个Connection对应一个Read协调
	OnClose(net.Conn)            // 被Read协程调用，每个Connection对应一个Read协调
}
