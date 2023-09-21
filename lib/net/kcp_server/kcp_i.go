package kcp_server

import (
	Kcp "github.com/xtaci/kcp-go/v5"
)

type IKcpSvrEventHandler interface {
	OnConn(*Kcp.UDPSession)             // 被Listener协程调用，一个KcpSvr对应一个Listener协程
	OnRead(*Kcp.UDPSession, []byte) int // 被Read协程调用，每个Connection对应一个Read协调
	OnClose(*Kcp.UDPSession)            // 被Read协程调用，每个Connection对应一个Read协调
}
