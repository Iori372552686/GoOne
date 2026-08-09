package connsvr

import (
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// handleClientPacket 是三种传输（TCP/WS/KCP）共用的客户端包处理逻辑：
// 解析 CS 头 → 会话校验/重绑定 → 经 router 转发到后端服务。
// 在各自读协程/事件循环内同步调用，不得保留 data 引用。
func handleClientPacket(gw net_mgr.GatewayServer, transport string, conn net.Conn, data []byte) {
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	if logger.DebugEnabled() {
		logger.Debugf("onClientPacket(%s): {dataLen: %v, headerLen: %v, remoteAddr: %v}",
			transport, len(data), headerLen, conn.RemoteAddr())
	}

	packetHeader := sharedstruct.CSPacketHeader{}
	if len(data) < packetHeader.Size() {
		logger.Errorf("Received datalen < packetHeader, packet is invalid (%s)", transport)
		return
	}

	packetHeader.From(data)
	packetBody := data[headerLen:]
	if logger.DebugEnabled() {
		logger.CmdDebugf(packetHeader.Cmd, "[uid: %d] Received client packet(%s): %#v", packetHeader.Uid, transport, packetHeader)
	}

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}

	// --- Default path: forward to backend server via router ---
	uid := packetHeader.Uid
	if uid == 0 {
		logger.Errorf("uid==0 and no client packet handler registered for cmd %d (%s)", packetHeader.Cmd, transport)
		return
	}

	client := gw.GetClientByUid(uid)
	if client == nil {
		// 首次登录：该 uid 的会话尚未绑定到当前连接，先建立绑定。
		// （GoOne 登录模型：uid 由外部预分配，客户端首包即携带 uid，
		// connsvr 据此建立 uid↔conn 映射，后续请求才能路由与回包。）
		// 登录限速。admission 拒绝（enforce 模式超 login_rate）时丢弃首包，
		// 不建立绑定。
		if a := globals.SessionHub.Admission(); a != nil && !a.TryAdmitLogin() {
			logger.Warningf("login rejected by admission (uid: %d, transport: %s)", uid, transport)
			return
		}
		client = gw.UpdateClientByUid(conn, uid, packetHeader.AppVersion)
		if client == nil {
			logger.Errorf("Failed to bind %s conn for uid: %v", transport, uid)
			return
		}
	} else if client.Conn != conn {
		// uid 已绑定到其它连接（重连/多地登录）：更新到当前连接。
		gw.UpdateClientByUid(conn, uid, client.Zone)
	}

	router.SendMsgByConn(uid, uid, client.Zone, packetHeader.Cmd, 0, packetBody, client.Ip, client.Port)
}

// proc tcp packet
func onTcpPacket(conn net.Conn, data []byte) {
	handleClientPacket(globals.ConnTcpSvr, "tcp", conn, data)
}

// proc WebSocket packet
func onWebSocketPacket(conn net.Conn, data []byte) {
	handleClientPacket(globals.ConnWsSvr, "ws", conn, data)
}

// proc kcp packet
func onKcpPacket(conn net.Conn, data []byte) {
	handleClientPacket(globals.ConnKcpSvr, "kcp", conn, data)
}

// busMsg proc cb func
func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	if misc.IsClientCmd(packet.Header.Cmd) {
		csPacketHeader := sharedstruct.CSPacketHeader{
			Uid:     packet.Header.Uid,
			Cmd:     packet.Header.Cmd,
			BodyLen: packet.Header.BodyLen,
		}

		// 头编码到栈上数组，避免每个下行包一次堆分配。
		var headerBuf [28]byte
		csPacketHeader.To(headerBuf[:])

		// 同一个 uid 只会绑定在一种传输通道上：按 TCP → WS → KCP 依次回退。
		if err := globals.ConnTcpSvr.SendByUid(packet.Header.Uid, headerBuf[:], packet.Body); err == nil {
			return
		}
		if err := globals.ConnWsSvr.SendByUid(packet.Header.Uid, headerBuf[:], packet.Body); err == nil {
			return
		}
		if err := globals.ConnKcpSvr.SendByUid(packet.Header.Uid, headerBuf[:], packet.Body); err != nil {
			logger.Debugf("downstream packet dropped, uid not on tcp/ws/kcp {uid:%v, cmd:%v}",
				packet.Header.Uid, packet.Header.Cmd)
		}
	} else if packet.Header.Cmd == uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ) {
		//onSSPacketConnKickout(packet)
	} else {
		globals.TransMgr.ProcessSSPacket(packet)
		packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
	}
}
