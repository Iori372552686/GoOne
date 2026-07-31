package net_mgr

import (
	"fmt"
	"net"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	"github.com/Iori372552686/GoOne/module/misc"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"

	"github.com/golang/protobuf/proto"
)

func (t *ConnWsTcpSvr) initAndRun(implType, mode string, port int, cb func(conn net.Conn, data []byte)) error {
	t.uidConnMap = make(map[uint64]*Client)
	t.connUidMap = make(map[net.Conn]uint64)
	t.remoteAddrConnMap = make(map[string]net.Conn)
	t.remoteAddrKickMap = make(map[string]bool)
	t.handler = cb

	return t.WsTcpSvr.InitAndRun(implType, mode, port, t)
}

func (t *ConnWsTcpSvr) OnConn(conn net.Conn) {
	// V3-P1-01：过载保护。
	if t.hub != nil {
		if a := t.hub.Admission(); a != nil && !a.TryAdmitConnection() {
			observeGatewayEvent("ws", "rejected")
			_ = conn.Close()
			return
		}
	}
	logger.Infof("new conn: %s", conn.RemoteAddr().String())
	observeGatewayEvent("ws", "accepted")
	if t.hub != nil {
		t.hub.IncConnection()
	}
}

// OnRead 在读协程内同步执行 handler：保证同连接消息顺序并提供天然背压。
func (self *ConnWsTcpSvr) OnRead(conn net.Conn, data []byte) int {
	safego.SafeFunc(func() { self.handler(conn, data) })
	return 0
}

func (t *ConnWsTcpSvr) OnClose(conn net.Conn) {
	observeGatewayEvent("ws", "closed")
	uid := t.removeConn(conn)
	if t.hub != nil {
		t.hub.DecConnection()
	}
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
}

func (t *ConnWsTcpSvr) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
	// P0-05：hub 路径锁内取不可变 Client，锁外写。
	if t.hub != nil {
		client := t.hub.ClientForSend(uid)
		if client == nil {
			logger.Debugf("uid doesn't exist {uid: %v}", uid)
			return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
		}
		if err := t.WriteData(client.Conn, data1, data2); err != nil {
			client.Conn.Close()
			observeGatewayEvent("ws", "write_error")
			logger.Errorf("Closed connection for failing to write data {uid: %v}| %v", uid, err)
			return err
		}
		if logger.DebugEnabled() {
			logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
		}
		return nil
	}

	t.lock.RLock()
	defer t.lock.RUnlock()

	conn, exists := t.uidConnMap[uid]
	if !exists {
		logger.Debugf("uid doesn't exist {uid: %v}", uid)
		return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
	}

	err := t.WriteData(conn.Conn, data1, data2)
	if err != nil {
		conn.Conn.Close()
		observeGatewayEvent("ws", "write_error")
		logger.Errorf("Closed connection for failing to write data {uid: %v}| %v", uid, err)
		return err
	}

	if logger.DebugEnabled() {
		logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	}
	return nil
}

// BroadcastByZone 向指定 zone 的所有在线客户端广播；zone <= 0 表示全服广播。
// P0-05：hub 路径锁内快照、锁外写。
func (t *ConnWsTcpSvr) BroadcastByZone(zone int32, data1 []byte, data2 []byte) {
	if t.hub != nil {
		clients := t.hub.SnapshotByZone(zone)
		for _, client := range clients {
			if err := t.WriteData(client.Conn, data1, data2); err != nil {
				client.Conn.Close()
				observeGatewayEvent("ws", "write_error")
				logger.Errorf("Closed connection for failing to write data {uid: %v} | %v", client.Uid, err)
			}
		}
		return
	}

	t.lock.RLock()
	defer t.lock.RUnlock()

	for _, client := range t.uidConnMap {
		if zone > 0 && client.Zone != uint32(zone) {
			continue
		}
		err := t.WriteData(client.Conn, data1, data2)
		if err != nil {
			client.Conn.Close()
			observeGatewayEvent("ws", "write_error")
			logger.Errorf("Closed connection for failing to write data {uid: %v} | %v", client.Uid, err)
			continue
		}
	}
}

func (t *ConnWsTcpSvr) Kick(uid uint64, reason g1_protocol.EKickOutReason) {
	if t.hub != nil {
		client := t.hub.ClientForSend(uid)
		if client == nil {
			logger.Infof("Can't find conn to kick. {uid:%v, reason:%v}", uid, reason)
			return
		}
		t.kick(client.Conn, uid, reason)
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	conn := t.uidConnMap[uid]
	if conn == nil {
		logger.Infof("Can't find conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}

	t.kick(conn.Conn, uid, reason)
}

func (t *ConnWsTcpSvr) KickByRemoteAddr(uid uint64, reason g1_protocol.EKickOutReason, remoteAddr string) {
	if t.hub != nil {
		conn := t.hub.ConnByRemoteAddr(remoteAddr)
		if conn == nil {
			logger.Infof("Cann't find conn to kick. {uid:%v, reason:%v}", uid, reason)
			return
		}
		t.hub.MarkKick(remoteAddr)
		t.kick(conn, uid, reason)
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	conn := t.remoteAddrConnMap[remoteAddr]
	if conn == nil {
		logger.Infof("Cann't find conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}
	t.remoteAddrKickMap[remoteAddr] = true

	t.kick(conn, uid, reason)
}

func (t *ConnWsTcpSvr) removeConn(conn net.Conn) uint64 {
	if t.hub != nil {
		uid, kicked := t.hub.RemoveConn(conn)
		if kicked {
			return 0
		}
		return uid
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	uid, exists := t.connUidMap[conn]
	if !exists {
		logger.Errorf("Can't find this conn from connUidMap{IP: %v}", conn.RemoteAddr())
		return 0
	}

	// 把连接与UID的对应关系删了
	delete(t.remoteAddrConnMap, conn.RemoteAddr().String())
	delete(t.connUidMap, conn)
	if connInMap, exists := t.uidConnMap[uid]; exists && connInMap.Conn == conn {
		delete(t.uidConnMap, uid)
		if t.remoteAddrKickMap[conn.RemoteAddr().String()] {
			delete(t.remoteAddrKickMap, conn.RemoteAddr().String())
			return 0
		}
	} else { // uid并不属于这个conn。在多地登录时，会出现。
		return 0
	}

	return uid
}

func (t *ConnWsTcpSvr) kick(conn net.Conn, uid uint64, reason g1_protocol.EKickOutReason) {
	defer t.Close(conn)
	observeGatewayEvent("ws", "kick")

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
	err = t.WriteData(conn, header.ToBytes(), msgData)
	if err != nil {
		observeGatewayEvent("ws", "write_error")
		logger.Errorf("Failed to write data in kick | %v", err)
		return
	}
}

func (t *ConnWsTcpSvr) UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *Client {
	// P0-05：hub 路径原子绑定 + IPv6 兼容 + 锁外 kick 旧连接。
	if t.hub != nil {
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

	oldCli := t.GetClientByUid(uid)
	ipAddr := strings.Split(conn.RemoteAddr().String(), ":")
	ip, port := ipAddr[0], ipAddr[1]

	newIns := &Client{
		Uid:        uid,
		Zone:       zone,
		Conn:       conn,
		RemoteAddr: conn.RemoteAddr().String(),
		Ip:         bus.IpStringToInt(ip),
		Port:       uint32(convert.StrToInt(port)),
	}

	t.lock.Lock()
	t.connUidMap[conn] = uid
	t.uidConnMap[uid] = newIns
	t.remoteAddrConnMap[conn.RemoteAddr().String()] = conn
	t.lock.Unlock()

	if oldCli != nil {
		t.kick(oldCli.Conn, uid, g1_protocol.EKickOutReason_MULTI_PLACE_LOGIN)
	}

	return newIns
}

func (t *ConnWsTcpSvr) GetClientByUid(uid uint64) *Client {
	if t.hub != nil {
		return t.hub.GetClientByUid(uid)
	}
	t.lock.RLock()
	client := t.uidConnMap[uid]
	t.lock.RUnlock()

	return client
}
