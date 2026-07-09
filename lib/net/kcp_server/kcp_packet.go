package kcp_server

import (
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// KcpPacketSvr splits the KCP byte stream into frames using KcpPacketInfo
// (mirrors tcp_server.TcpPacketSvr). Handlers receive complete packets.
type KcpPacketSvr struct {
	KcpSvr

	packetInfo KcpPacketInfo
	handler    IKcpPacketSvrEventHandler
}

func (s *KcpPacketSvr) InitAndRun(ip string, port int, packetInfo KcpPacketInfo, handler IKcpPacketSvrEventHandler) error {
	s.packetInfo = packetInfo
	s.handler = handler
	return s.KcpSvr.InitAndRun(ip, port, s)
}

func (s *KcpPacketSvr) OnConn(conn net.Conn) {
	s.handler.OnConn(conn)
}

func (s *KcpPacketSvr) OnRead(conn net.Conn, data []byte) int {
	dataLen := len(data)
	headerLen := s.packetInfo.HeaderLen
	consumed := 0
	for { // There likely be more than one packet
		if dataLen >= consumed+headerLen { // header is ready
			bodyLen := s.packetInfo.BodyLen(data[consumed : consumed+headerLen])
			if bodyLen < 0 {
				logger.Warningf("Received an invalid kcp packet header, closing connection {remote:%v}", conn.RemoteAddr())
				conn.Close()
				return dataLen
			}
			if dataLen >= consumed+headerLen+bodyLen { // header and body is ready
				s.handler.OnPacket(conn, data[consumed:consumed+headerLen+bodyLen])
				consumed += headerLen + bodyLen
			} else {
				return consumed
			}
		} else {
			return consumed
		}
	}
}

func (s *KcpPacketSvr) OnClose(conn net.Conn) {
	s.handler.OnClose(conn)
}
