package net_mgr

import (
	"fmt"
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/net/kcp_server"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/misc"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"

	"github.com/golang/protobuf/proto"
)

func (t *ConnKcpSvr) initAndRun(ip string, port int, cb func(conn net.Conn, data []byte)) error {
	// V3-P1-02：本地 map 路径已删除，会话状态由 hub 拥有；这里只装配 handler。
	t.handler = cb

	packetInfo := kcp_server.KcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}

	return t.KcpPacketSvr.InitAndRun(ip, port, packetInfo, t)
}

// 被Listener协程调用，一个KcpSvr对应一个Listener协程
func (t *ConnKcpSvr) OnConn(conn net.Conn) {
	// V3-P1-01：过载保护。
	if t.hub != nil {
		if a := t.hub.Admission(); a != nil && !a.TryAdmitConnection() {
			observeGatewayEvent("kcp", "rejected")
			_ = conn.Close()
			return
		}
	}
	logger.Infof("kcp new conn: %s", conn.RemoteAddr().String())
	observeGatewayEvent("kcp", "accepted")
	if t.hub != nil {
		t.hub.IncConnection()
	}
}

// 被Read协程调用。handler 在读协程内同步执行：保证同连接消息顺序、
// 天然背压，且 data（读缓冲别名）在返回前使用完毕，无需拷贝。
// handler 内部不得保留 data 引用。
func (t *ConnKcpSvr) OnPacket(conn net.Conn, data []byte) {
	safego.SafeFunc(func() { t.handler(conn, data) })
}

// 被Read协程调用
func (t *ConnKcpSvr) OnClose(conn net.Conn) {
	observeGatewayEvent("kcp", "closed")
	uid := t.removeConn(conn)
	if t.hub != nil {
		t.hub.DecConnection()
	}
	if uid == 0 {
		return
	}

	logger.Infof("kcp client close {RemoteIp: %v, Uid: %v}", conn.RemoteAddr(), uid)

	// 给mainsvr发登出包
	req := g1_protocol.LogoutReq{}
	req.ByServer = true
	req.Reason = "disconnect"
	err := router.SendPbMsgBySvrTypeSimple(uint32(misc.ServerType_MainSvr), uid, 0, g1_protocol.CMD_MAIN_LOGOUT_REQ, &req)
	if err != nil {
		logger.Error(err)
	}
}

func (t *ConnKcpSvr) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
	// P0-05：锁内取 Client，锁外写。
	client := t.hub.ClientForSend(uid)
	if client == nil {
		logger.Debugf("uid doesn't exist {uid: %v}", uid)
		return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
	}
	if err := t.WriteData(client.Conn, data1, data2); err != nil {
		client.Conn.Close()
		observeGatewayEvent("kcp", "write_error")
		logger.Errorf("Closed kcp connection for failing to write data {uid: %v} | %v", uid, err)
		return err
	}
	if logger.DebugEnabled() {
		logger.Debugf("Send to kcp client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	}
	return nil
}

// BroadcastByZone 向指定 zone 的所有在线客户端广播；zone <= 0 表示全服广播。
// P0-05：锁内快照、锁外写。
func (t *ConnKcpSvr) BroadcastByZone(zone int32, data1 []byte, data2 []byte) {
	clients := t.hub.SnapshotByZone(zone)
	for _, client := range clients {
		if err := t.WriteData(client.Conn, data1, data2); err != nil {
			client.Conn.Close()
			observeGatewayEvent("kcp", "write_error")
			logger.Errorf("Closed kcp connection for failing to write data {uid: %v} | %v", client.Uid, err)
		}
	}
}

func (t *ConnKcpSvr) Kick(uid uint64, reason g1_protocol.EKickOutReason) {
	client := t.hub.ClientForSend(uid)
	if client == nil {
		logger.Infof("Can't find kcp conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}
	t.kick(client.Conn, uid, reason)
}

func (t *ConnKcpSvr) KickByRemoteAddr(uid uint64, reason g1_protocol.EKickOutReason, remoteAddr string) {
	conn := t.hub.ConnByRemoteAddr(remoteAddr)
	if conn == nil {
		logger.Infof("Can't find kcp conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}
	t.hub.MarkKick(remoteAddr)
	t.kick(conn, uid, reason)
}

func (t *ConnKcpSvr) removeConn(conn net.Conn) uint64 {
	uid, kicked := t.hub.RemoveConn(conn)
	if kicked {
		return 0
	}
	return uid
}

func (t *ConnKcpSvr) kick(conn net.Conn, uid uint64, reason g1_protocol.EKickOutReason) {
	defer t.Close(conn)
	observeGatewayEvent("kcp", "kick")

	logger.Infof("Kick out kcp client {uid:%v, reason:%v, ip:%v}", uid, reason, conn.RemoteAddr())

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
	if err := t.WriteData(conn, headerBuf[:], msgData); err != nil {
		observeGatewayEvent("kcp", "write_error")
		logger.Errorf("Failed to write kcp data in kick | %v", err)
	}
}

func (t *ConnKcpSvr) UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *Client {
	// P0-05：原子绑定 + IPv6 兼容 + 锁外 kick 旧连接。
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

func (t *ConnKcpSvr) GetClientByUid(uid uint64) *Client {
	return t.hub.GetClientByUid(uid)
}
