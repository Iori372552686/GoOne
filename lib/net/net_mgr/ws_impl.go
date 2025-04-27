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
	g1_protocol "github.com/Iori372552686/game_protocol"

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
	logger.Infof("new conn: %s", conn.RemoteAddr().String())
}

func (self *ConnWsTcpSvr) OnRead(conn net.Conn, data []byte) int {
	safego.Go(func() { self.handler(conn, data) })
	return 0
}

func (t *ConnWsTcpSvr) OnClose(conn net.Conn) {
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
}

func (t *ConnWsTcpSvr) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
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
		logger.Errorf("Closed connection for failing to write data {uid: %v}| %v", uid, err)
		return err
	}

	logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	return nil
}

func (t *ConnWsTcpSvr) BroadcastByZone(zone int32, data1 []byte, data2 []byte) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for _, conn := range t.uidConnMap {
		// TODO check zone
		err := t.WriteData(conn.Conn, data1, data2)
		if err != nil {
			conn.Conn.Close()
			logger.Errorf("Closed connection for failing to write data {uid: %v}| %v\", uid, err")
			continue
		}
	}
}

func (t *ConnWsTcpSvr) Kick(uid uint64, reason g1_protocol.EKickOutReason) {
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
		logger.Errorf("Failed to write data in kick | %v", err)
		return
	}
}

func (t *ConnWsTcpSvr) UpdateClientByUid(conn net.Conn, uid uint64, zone uint32) *Client {
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
	t.lock.RLock()
	client := t.uidConnMap[uid]
	t.lock.RUnlock()

	return client
}
