package net_mgr

import (
	"GoOne/lib/api/logger"
	Kcp "github.com/xtaci/kcp-go/v5"
)

/**
* @Description:
* @receiver: self
* @param: port
* @param: cb
* @return: error
* @Author: Iori
* @Date: 2022-02-15 14:48:10
**/
func (self *ConnKcpSvr) InitAndRun(port int, cb func(conn *Kcp.UDPSession, data []byte)) error {
	self.uidConnMap = make(map[uint64]*Kcp.UDPSession)
	self.connUidMap = make(map[*Kcp.UDPSession]uint64)
	self.remoteAddrConnMap = make(map[string]*Kcp.UDPSession)
	self.remoteAddrKickMap = make(map[string]bool)
	self.handler = cb

	return self.KcpSvr.InitAndRun(port, self)
}

/**
* @Description:
* @receiver: self
* @param: conn
* @Author: Iori
* @Date: 2022-02-15 14:48:13
**/
func (self *ConnKcpSvr) OnConn(conn *Kcp.UDPSession) {
	logger.Infof("kcp new conn: %s", conn.RemoteAddr().String())
	return
}

/**
* @Description:
* @receiver: self
* @param: conn
* @param: data
* @return: int
* @Author: Iori
* @Date: 2022-02-15 14:48:15
**/
func (self *ConnKcpSvr) OnRead(conn *Kcp.UDPSession, data []byte) int {
	go self.handler(conn, data)
	return 0
}

/**
* @Description:
* @receiver: self
* @param: conn
* @Author: Iori
* @Date: 2022-02-15 14:48:18
**/
func (self *ConnKcpSvr) OnClose(conn *Kcp.UDPSession) {
	logger.Infof("kcp client close {RemoteIp: %v}", conn.RemoteAddr())

	self.removeConn(conn)
	return
}

/**
* @Description:
* @receiver: self
* @param: conn
* @return: uint64
* @Author: Iori
* @Date: 2022-02-15 14:48:19
**/
func (self *ConnKcpSvr) removeConn(conn *Kcp.UDPSession) uint64 {
	self.lock.Lock()
	defer self.lock.Unlock()

	uid, exists := self.connUidMap[conn]
	if !exists {
		logger.Errorf("Can't find this kcp conn from connUidMap{IP: %v}", conn.RemoteAddr())
		return 0
	}

	// 把连接与UID的对应关系删了
	delete(self.remoteAddrConnMap, conn.RemoteAddr().String())
	delete(self.connUidMap, conn)
	if connInMap, exists := self.uidConnMap[uid]; exists && connInMap == conn {
		delete(self.uidConnMap, uid)
		if self.remoteAddrKickMap[conn.RemoteAddr().String()] {
			delete(self.remoteAddrKickMap, conn.RemoteAddr().String())
			return 0
		}
	} else {
		return 0
	}

	return uid
}
