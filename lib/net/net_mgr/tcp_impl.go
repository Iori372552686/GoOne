package net_mgr

import (
	"fmt"
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	gnet_svr "github.com/Iori372552686/GoOne/lib/net/gnet_server"
	"github.com/Iori372552686/GoOne/lib/net/tcp_server"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/misc"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"

	"github.com/golang/protobuf/proto"
)

func (t *ConnTcpSvr) initAndRun(ip string, port int, cb func(conn net.Conn, data []byte)) error {
	t.initSessionMaps(cb)
	t.transport = &t.TcpPacketSvr

	packetInfo := tcp_server.TcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}

	return t.TcpPacketSvr.InitAndRun(ip, port, packetInfo, t)
}

// initAndRunGnet starts the event-driven gnet backend: no per-connection
// goroutines, framing happens inside the event loop, downlink writes go
// through AsyncWrite. Session semantics are identical to the gonet backend.
func (t *ConnTcpSvr) initAndRunGnet(ip string, port int, cb func(conn net.Conn, data []byte)) error {
	t.initSessionMaps(cb)

	gs := gnet_svr.NewTcpServer(
		sharedstruct.ByteLenOfCSPacketHeader(),
		sharedstruct.ByteLenOfCSPacketBody,
		t,
	)
	t.transport = gs
	return gs.Start(ip, port)
}

func (t *ConnTcpSvr) initSessionMaps(cb func(conn net.Conn, data []byte)) {
	// 本地 map 路径已删除，会话状态由 hub 拥有；这里只装配 handler。
	t.handler = cb
}

// 被Listener协程调用，一个TcpSvr对应一个Listener协程
func (t *ConnTcpSvr) OnConn(conn net.Conn) {
	// admission 用原子 TryAcquireConnection 占用名额；拒绝（enforce 超限/排空期）
	// 时立即关闭连接，不增加计数，且不标记 admitted，使 OnClose 不释放。
	t.ensureLease()
	if t.hub != nil {
		if a := t.hub.Admission(); a != nil && !a.TryAcquireConnection() {
			observeGatewayEvent("tcp", "rejected")
			_ = conn.Close()
			return
		}
	}
	logger.Infof("new conn: %s", conn.RemoteAddr().String())
	observeGatewayEvent("tcp", "accepted")
	// 仅在 admission 通过后记录 admitted 并增加底层连接计数。
	t.lease.markAdmitted(conn)
	if t.hub != nil {
		t.hub.IncConnection()
	}
}

// 被Read协程调用，每个Connection对应一个Read协调。
// handler 在读协程内同步执行：保证同连接消息顺序、天然背压，且 data
// （读缓冲别名）在返回前使用完毕，无需拷贝。handler 内部不得保留 data 引用。
func (t *ConnTcpSvr) OnPacket(conn net.Conn, data []byte) {
	safego.SafeFunc(func() { t.handler(conn, data) })
}

// 被Read协程调用，每个Connection对应一个Read协调
func (t *ConnTcpSvr) OnClose(conn net.Conn) {
	observeGatewayEvent("tcp", "closed")
	// 仅为 admitted 连接释放计数与 admission 名额，避免被拒绝的连接误减计数。
	if t.lease != nil && t.lease.takeIfAdmitted(conn) {
		if t.hub != nil {
			t.hub.DecConnection()
		}
		if a := admissionOf(t.hub); a != nil {
			a.ReleaseConnection()
		}
	}
	uid := t.removeConn(conn)
	if uid == 0 {
		return
	}

	logger.Infof("client close {RemoteIp: %v, Uid: %v}", conn.RemoteAddr(), uid)

	// 给mainsvr发登出包
	req := g1_protocol.LogoutReq{}
	req.ByServer = true
	req.Reason = "disconnect"
	err := router.SendPbMsgBySvrTypeSimple(uint32(misc.ServerType_MainSvr), uid, 0, g1_protocol.CMD_MAIN_LOGOUT_REQ, &req)
	if err != nil {
		logger.Error(err)
	}
	// todo: 如果client已经下线了，可能会再被拉起来处理一次这个消息。
}

func (t *ConnTcpSvr) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
	// 锁内取不可变 Client，锁外写网络。
	client := t.hub.ClientForSend(uid)
	if client == nil {
		logger.Debugf("uid doesn't exist {uid: %v}", uid)
		return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
	}
	if err := t.transport.WriteData(client.Conn, data1, data2); err != nil {
		client.Conn.Close()
		observeGatewayEvent("tcp", "write_error")
		logger.Errorf("Closed connection for failing to write data {uid: %v}| %v", uid, err)
		return err
	}
	if logger.DebugEnabled() {
		logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	}
	return nil
}

// BroadcastByZone 向指定 zone 的所有在线客户端广播；zone <= 0 表示全服广播。
//
// 锁内构造目标快照，锁外逐个写——一个慢连接不得阻塞其它连接发送。
func (t *ConnTcpSvr) BroadcastByZone(zone int32, data1 []byte, data2 []byte) {
	clients := t.hub.SnapshotByZone(zone)
	for _, client := range clients {
		if err := t.transport.WriteData(client.Conn, data1, data2); err != nil {
			client.Conn.Close()
			observeGatewayEvent("tcp", "write_error")
			logger.Errorf("Closed connection for failing to write data {uid: %v} | %v", client.Uid, err)
		}
	}
}

func (t *ConnTcpSvr) Kick(uid uint64, reason g1_protocol.EKickOutReason) {
	// 锁内取 conn，锁外 marshal/write/close。
	client := t.hub.ClientForSend(uid)
	if client == nil {
		logger.Infof("Can't find conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}
	t.kick(client.Conn, uid, reason)
}

func (t *ConnTcpSvr) KickByRemoteAddr(uid uint64, reason g1_protocol.EKickOutReason, remoteAddr string) {
	conn := t.hub.ConnByRemoteAddr(remoteAddr)
	if conn == nil {
		logger.Infof("Cann't find conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}
	t.hub.MarkKick(remoteAddr)
	t.kick(conn, uid, reason)
}

func (t *ConnTcpSvr) removeConn(conn net.Conn) uint64 {
	// 委托 hub.RemoveConn；返回 uid=0 表示未绑定或被替换（不触发登出）。
	// kicked=true 表示是 kick 导致的关闭，不触发登出包。
	uid, kicked := t.hub.RemoveConn(conn)
	if kicked {
		return 0
	}
	return uid
}

func (t *ConnTcpSvr) kick(conn net.Conn, uid uint64, reason g1_protocol.EKickOutReason) {
	defer t.transport.Close(conn)
	observeGatewayEvent("tcp", "kick")

	logger.Infof("Kick out client {uid:%v, reason:%v, ip:%v}", uid, reason, conn.RemoteAddr())

	msg := g1_protocol.ScKickOut{Reason: reason}
	msgData, err := proto.Marshal(&msg)
	if err != nil {
		logger.Errorf("Marshal error in ScKickOut | %v", err)
		return
	}

	header := sharedstruct.CSPacketHeader{
		Uid:     uid,
		Cmd:     uint32(g1_protocol.CMD_SC_KICK_OUT),
		BodyLen: uint32(len(msgData)),
	}
	var headerBuf [28]byte
	header.To(headerBuf[:])
	err = t.transport.WriteData(conn, headerBuf[:], msgData)
	if err != nil {
		observeGatewayEvent("tcp", "write_error")
		logger.Errorf("Failed to write data in kick | %v", err)
		return
	}
}

func (t *ConnTcpSvr) UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *Client {
	// 用 BindClient 原子完成查旧+写新索引，IPv6 兼容（net.SplitHostPort），
	// 锁外 kick 旧连接。
	newIns, oldCli, err := t.hub.BindClient(conn, uid, zone)
	if err != nil {
		logger.Errorf("BindClient failed {uid: %v}: %v", uid, err)
		return nil
	}
	if oldCli != nil {
		t.kick(oldCli.Conn, uid, g1_protocol.EKickOutReason_MULTI_PLACE_LOGIN)
	}
	return newIns
}

func (t *ConnTcpSvr) GetClientByUid(uid uint64) *Client {
	return t.hub.GetClientByUid(uid)
}
