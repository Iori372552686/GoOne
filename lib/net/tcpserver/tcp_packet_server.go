package tcpserver

import (
	"GoOne/lib/logger"
	"net"

)

type TcpPacketInfo struct {
	HeaderLen int
	BodyLen func([]byte) int
}

type ITcpPacketSvrEventHandler interface {
	OnConn(net.Conn)  // 被Listener协程调用，一个TcpPacketSvr对应一个Listener协程
	OnPacket(net.Conn, []byte)  // 被Read协程调用，每个Connection对应一个Read协调
	OnClose(net.Conn)  // 被Read协程调用，每个Connection对应一个Read协调
}


type TcpPacketSvr struct {
	TcpSvr

	packetInfo TcpPacketInfo
	handler    ITcpPacketSvrEventHandler
}

func (s *TcpPacketSvr) InitAndRun(ip string, port int, packetInfo TcpPacketInfo, handler ITcpPacketSvrEventHandler) error {
	s.packetInfo = packetInfo
	s.handler = handler
	return s.TcpSvr.InitAndRun(ip, port, s)
}

func (s *TcpPacketSvr) OnConn(conn net.Conn) {
	s.handler.OnConn(conn)
}

func (s *TcpPacketSvr) OnRead(conn net.Conn, data []byte) int {
	dataLen := len(data)
	headerLen := s.packetInfo.HeaderLen
	logger.Infof("on read, len=%d, headlen=%d", dataLen, headerLen)
	consumed := 0
	for {  // There likely be more than one packet
		if dataLen >= consumed + headerLen {  // header is ready
			bodyLen := s.packetInfo.BodyLen(data[consumed : consumed + headerLen])
			if dataLen >= consumed + headerLen + bodyLen {  // header and body is ready
				s.handler.OnPacket(conn, data[consumed : consumed + headerLen + bodyLen])
				consumed += headerLen + bodyLen
			} else {
				return consumed
			}
		} else {
			return consumed
		}
	}
}


func (s *TcpPacketSvr) OnClose(conn net.Conn) {
	s.handler.OnClose(conn)
}
