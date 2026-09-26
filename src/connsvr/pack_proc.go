package connsvr

import (
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/login"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// isDevMode 报告当前运行环境是否为 dev。仅影响网关接入策略：
// dev 放行预分配 uid 模型与 GM 命令；非 dev 只接受经账号服认证的登录握手。
func isDevMode() bool {
	return conf.Get("base_cfg.runtime.env_mode").String() == "dev"
}

// handleClientPacket 是三种传输（TCP/WS/KCP）共用的客户端包处理逻辑：
// 解析 CS 头 → 会话校验/认证绑定 → 经 router 转发到后端服务。
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
		// GM 命令（类型 0xa）默认同样被网关拒绝；dev 模式放行供 tester 联调回归。
		if !(misc.IsGmCmd(packetHeader.Cmd) && isDevMode()) {
			logger.Debugf("Received an inner command from client: %#v", packetHeader)
			return
		}
	}

	// 连接优先：已绑定会话的连接一律以会话身份路由；包头自报 UID 与会话不符时
	// 拒绝，防止已认证连接被跨 UID 重绑冒用。
	if client := gw.GetClientByConn(conn); client != nil {
		if packetHeader.Uid != 0 && packetHeader.Uid != client.Uid {
			logger.Errorf("packet uid %d mismatches bound session uid %d, rejected {cmd: %d, transport: %s}",
				packetHeader.Uid, client.Uid, packetHeader.Cmd, transport)
			return
		}
		router.SendMsgByConn(client.Uid, client.Uid, client.Zone, packetHeader.Cmd, 0, packetBody, client.Ip, client.Port)
		return
	}

	// --- 未绑定连接：只有登录握手（或 dev 预分配模型）允许建立绑定 ---
	bindUid := uint64(0)
	if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
		req := &g1_protocol.LoginReq{}
		if err := proto.Unmarshal(packetBody, req); err != nil {
			logger.Errorf("LoginReq unmarshal failed {uid: %d, transport: %s, err: %v}", packetHeader.Uid, transport, err)
			return
		}
		if req.GetAccount() == "" || req.GetChannelId() == 0 {
			logger.Errorf("LoginReq --> account or ChannelId error, uid=%d (%s)", packetHeader.Uid, transport)
			return
		}
		ret, accUid := login.OnCheckAuthByAccSvr(req.GetAccount(), req.GetToken(), req.GetChannelId(), req.GetLoginType())
		if !ret {
			// 认证失败：丢弃首包，不建立绑定（dev 模式不会走到这里）。
			logger.Errorf("login auth rejected {account: %s, uid: %d, transport: %s}", req.GetAccount(), packetHeader.Uid, transport)
			return
		}
		if accUid > 0 {
			// 认证所得 uid 优先于客户端自报 uid：身份由账号服决定。
			bindUid = accUid
		} else {
			// dev 回退：账号服跳过且账号非纯数字，沿用客户端自报 uid
			//（外部预分配模型，tester/stress 依赖）。
			bindUid = packetHeader.Uid
		}
	} else if isDevMode() && packetHeader.Uid != 0 {
		// dev 预分配模型：首包即携带 uid，直接绑定。
		bindUid = packetHeader.Uid
	} else {
		// 非 dev：未认证连接必须先走登录握手，业务包不得自报 uid 建立绑定。
		logger.Errorf("unauthenticated conn must login first {cmd: %d, uid: %d, transport: %s}", packetHeader.Cmd, packetHeader.Uid, transport)
		return
	}

	if bindUid == 0 {
		logger.Errorf("uid==0 and no client packet handler registered for cmd %d (%s)", packetHeader.Cmd, transport)
		return
	}

	// 登录限速。admission 拒绝（enforce 模式超 login_rate）时丢弃首包，不建立绑定。
	if a := globals.SessionHub.Admission(); a != nil && !a.TryAdmitLogin() {
		logger.Warningf("login rejected by admission (uid: %d, transport: %s)", bindUid, transport)
		return
	}
	client := gw.UpdateClientByUid(conn, bindUid, packetHeader.AppVersion)
	if client == nil {
		logger.Errorf("Failed to bind %s conn for uid: %v", transport, bindUid)
		return
	}

	router.SendMsgByConn(bindUid, bindUid, client.Zone, packetHeader.Cmd, 0, packetBody, client.Ip, client.Port)
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
	// GM 响应（类型 0xa）与客户端命令一样需要下发；与上行 GM 放行策略保持一致
	//（PokerGo 同款：仅放行 GM 响应的下行，上行放行见 handleClientPacket）。
	if misc.IsClientCmd(packet.Header.Cmd) || misc.IsGmCmd(packet.Header.Cmd) {
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
	} else {
		// 其余命令（含 CMD_CONN_KICK_OUT_REQ：由 IDL 注册的 ConnService.KickOut
		// 在 TransMgr 内处理）进入事务分发。历史代码曾在此拦截 kick 命令并
		// 调用注释掉的本地 handler，导致服务端踢人包被吞、会话永不踢断。
		globals.TransMgr.ProcessSSPacket(packet)
		packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
	}
}
