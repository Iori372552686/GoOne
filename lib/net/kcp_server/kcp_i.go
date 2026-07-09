package kcp_server

import (
	"net"
)

// IKcpSvrEventHandler receives raw-stream events from KcpSvr.
// conn is the underlying *kcp.UDPSession exposed as net.Conn so the session
// layer can share code with the TCP/WS gateways.
type IKcpSvrEventHandler interface {
	OnConn(net.Conn)             // 被Listener协程调用，一个KcpSvr对应一个Listener协程
	OnRead(net.Conn, []byte) int // 被Read协程调用；返回已消费字节数，未消费部分保留在粘包缓冲中
	OnClose(net.Conn)            // 被Read协程调用
}

// IKcpPacketSvrEventHandler receives framed packets from KcpPacketSvr.
type IKcpPacketSvrEventHandler interface {
	OnConn(net.Conn)
	OnPacket(net.Conn, []byte) // 在读协程内同步调用；不得保留 data 引用
	OnClose(net.Conn)
}

// KcpPacketInfo describes the frame layout used to split the KCP byte stream.
type KcpPacketInfo struct {
	HeaderLen int
	BodyLen   func(header []byte) int // 返回 body 长度；<0 表示非法包，连接将被关闭
}
