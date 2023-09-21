package tcp_server

import (
	"GoOne/common/misc"
	"GoOne/lib/api/logger"
	"GoOne/lib/api/sharedstruct"
	"GoOne/lib/net/tcpserver"
	"GoOne/lib/service/router"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/connsvr/config"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"net"
	"net/http"

	"strconv"
	"sync"
)

// 必须实现 tcpserver.ITcpPacketSvrEventHandler
type ConnTcpSvr struct {
	tcpserver.TcpPacketSvr

	uidConnMap        map[uint64]net.Conn
	connUidMap        map[net.Conn]uint64
	remoteAddrConnMap map[string]net.Conn
	remoteAddrKickMap map[string]bool
	lock              sync.RWMutex
}

func NewTcpSvr() *ConnTcpSvr {
	return &ConnTcpSvr{}
}

func (t *ConnTcpSvr) InitAndRun(ip string, port int) error {
	t.uidConnMap = make(map[uint64]net.Conn)
	t.connUidMap = make(map[net.Conn]uint64)
	t.remoteAddrConnMap = make(map[string]net.Conn)
	t.remoteAddrKickMap = make(map[string]bool)

	packetInfo := tcpserver.TcpPacketInfo{
		HeaderLen: sharedstruct.ByteLenOfCSPacketHeader(),
		BodyLen:   sharedstruct.ByteLenOfCSPacketBody,
	}

	return t.TcpPacketSvr.InitAndRun(ip, port, packetInfo, t)
}

// 被Listener协程调用，一个TcpSvr对应一个Listener协程
func (t *ConnTcpSvr) OnConn(conn net.Conn) {
	logger.Infof("new conn: %s", conn.RemoteAddr().String())
}

// 被Read协程调用，每个Connection对应一个Read协调
func (t *ConnTcpSvr) OnPacket(conn net.Conn, data []byte) {
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	logger.Debugf("on packet: {dataLen: %v, headerLen: %v, remoteAddr: %v}\n",
		len(data), headerLen, conn.RemoteAddr())

	packetHeader := sharedstruct.CSPacketHeader{}
	packetHeader.From(data)
	packetBody := data[headerLen:]
	logger.Debugf("[uid: %d] Received client packet: %#v", packetHeader.Uid, packetHeader)

	// 实现帐号认证系统之前，由client决定uid
	uid := packetHeader.Uid
	if uid > 0 {
		// t.lock.RLock()
		// oldConn := t.uidConnMap[uid]
		// t.lock.RUnlock()

		// if oldConn != conn {
		// 	t.lock.Lock()
		// 	oldConn = t.uidConnMap[uid]
		// 	t.uidConnMap[uid] = conn
		// 	t.connUidMap[conn] = uid
		// 	t.lock.Unlock()
		// 	if oldConn != nil {
		// 		t.kick(oldConn, uid, g1_protocol.EKickOutReason_MULTI_PLACE_LOGIN)
		// 	}
		// }

	} else {

		if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
			req := &g1_protocol.LoginReq{}
			err := proto.Unmarshal(packetBody, req)
			if err != nil {
				logger.Errorf(" Fail to unmarshal LoginReq | %v ", err)
				return
			}

			if req.GetAccount() == "" || req.GetServerId() == "" {
				logger.Errorf(" LoginReq  -- > account or ServerId   error !!! ")
				return
			}

			//http连接loginsvr 去做校验
			ret, aid := connLoginHttpCheck(req.Account, req.Password, req.ServerId)
			if !ret {
				return
			}

			//uid =  sid + aid
			int_id, _ := strconv.Atoi(req.GetServerId() + strconv.Itoa(aid))
			uid = uint64(int_id)

		}
	}

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}

	t.lock.Lock()
	t.uidConnMap[uid] = conn
	t.connUidMap[conn] = uid
	t.remoteAddrConnMap[conn.RemoteAddr().String()] = conn
	t.lock.Unlock()

	serverType := misc.ServerTypeInCmd(packetHeader.Cmd)
	// router.SendMsgBySvrType(serverType, uid, packetHeader.Cmd, 0, 0, packetBody)
	router.SendMsgBySvrTypeConn(serverType, uid, packetHeader.Cmd, 0, 0, packetBody, conn.RemoteAddr().String())
}

func connLoginHttpCheck(account string, password string, serverid string) (bool, int) {
	// 创建请求
	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		config.SvrCfg.LoginSdkAddr+"/userlogin_verify?account="+account+"&password="+password+"&curserid="+serverid,
		nil)
	if err != nil {
		return false, 0
	}

	req.Header.Add("token", "a")
	resp, err1 := client.Do(req)
	if err1 != nil {
		return false, 0
	}

	// 获取消息体
	body, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		return false, 0
	}

	fmt.Println(string(body))
	if resp.StatusCode == 200 {
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		if result["ret"] == "true" {
			return true, int(result["accountid"].(float64))
		}
	}

	return false, 0
}

// 被Read协程调用，每个Connection对应一个Read协调
func (t *ConnTcpSvr) OnClose(conn net.Conn) {
	logger.Infof("client close {RemoteIp: %v}", conn.RemoteAddr())

	uid := t.removeConn(conn)
	if uid == 0 {
		return
	}

	logger.Infof("client close {RemoteIp: %v, Uid: %v}", conn.RemoteAddr(), uid)

	// 给mainsvr发登出包
	req := g1_protocol.LogoutReq{}
	req.ByServer = true
	req.Reason = "disconnect"
	err := router.SendPbMsgBySvrTypeSimple(uint32(misc.ServerType_MainSvr), uid,
		uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ), &req)
	if err != nil {
		glog.Error(err)
	}
	// todo: 如果client已经下线了，可能会再被拉起来处理一次这个消息。
}

func (t *ConnTcpSvr) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
	t.lock.RLock()
	defer t.lock.RUnlock()

	conn, exists := t.uidConnMap[uid]
	if !exists {
		logger.Debugf("uid doesn't exist {uid: %v}", uid)
		return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
	}

	err := t.WriteData(conn, data1, data2)
	if err != nil {
		conn.Close()
		logger.Errorf("Closed connection for failing to write data {uid: %v}| %v", uid, err)
		return err
	}

	logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	return nil
}

func (t *ConnTcpSvr) BroadcastByZone(zone int32, data1 []byte, data2 []byte) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for _, conn := range t.uidConnMap {
		// TODO check zone
		err := t.WriteData(conn, data1, data2)
		if err != nil {
			conn.Close()
			logger.Errorf("Closed connection for failing to write data {uid: %v}| %v\", uid, err")
			continue
		}
	}
}

func (t *ConnTcpSvr) Kick(uid uint64, reason g1_protocol.EKickOutReason) {
	t.lock.Lock()
	defer t.lock.Unlock()

	conn := t.uidConnMap[uid]
	if conn == nil {
		logger.Infof("Can't find conn to kick. {uid:%v, reason:%v}", uid, reason)
		return
	}

	t.kick(conn, uid, reason)
}

func (t *ConnTcpSvr) KickByRemoteAddr(uid uint64, reason g1_protocol.EKickOutReason, remoteAddr string) {
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

func (t *ConnTcpSvr) removeConn(conn net.Conn) uint64 {
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
	if connInMap, exists := t.uidConnMap[uid]; exists && connInMap == conn {
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

func (t *ConnTcpSvr) kick(conn net.Conn, uid uint64, reason g1_protocol.EKickOutReason) {
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
