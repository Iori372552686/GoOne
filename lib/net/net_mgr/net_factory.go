package net_mgr

import (
	"errors"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	gnet_svr "github.com/Iori372552686/GoOne/lib/net/gnet_server"
	"net"

	"github.com/panjf2000/gnet"
	Kcp "github.com/xtaci/kcp-go/v5"
)

// tcp impl
func (self *ConnTcpSvr) CreateTcpServer(implType string, port int, cb func(conn net.Conn, data []byte)) error {
	logger.Infof(" -----  CreateTcpServer ---- implType =%s, port =%d", implType, port)
	if cb == nil || port == 0 {
		return errors.New("CreateTcpServer args fail ！")
	}

	switch implType {
	case "gev":
		//todo   -- need time, wait!
		return nil

	case "gnet":
		//todo   -- need time, wait!
		return nil

	default: //"gonet"
		return self.initAndRun("0.0.0.0", port, cb)
	}
}

// udp impl
func CreateUdpServer(implType string, port int, cb func(conn gnet.Conn, data []byte)) error {
	logger.Infof(" -----  CreateUdpServer ---- implType =%s, port =%d", implType, port)
	if port == 0 || implType == "" || cb == nil {
		return errors.New("CreateUdpServer args fail ！")
	}

	switch implType {
	case "gev":
		//todo   -- need you, wait!
		return nil

	case "gonet":
		//todo   -- need time, wait!
		return nil

	default: //"gnet"
		return gnet_svr.NewUdpServer(port, cb)
	}
}

// Kcp impl
func (self *ConnKcpSvr) CreateKcpServer(port int, cb func(conn *Kcp.UDPSession, data []byte)) error {
	logger.Infof(" -----  CreateKcpServer ----, port =%d", port)
	if port == 0 || cb == nil {
		return errors.New("CreateKcpServer error, Args fail ！")
	}

	err := self.InitAndRun(port, cb)
	if err != nil {
		logger.Errorf("CreateKcpServer InitAndRun ** fail ** !")
		return err
	}

	return nil
}

// websocket impl
func (self *ConnWsTcpSvr) CreateWebSocketServer(implType, mode string, port int, cb func(conn net.Conn, data []byte)) error {
	logger.Infof(" -----  CreateWebSocketServer ---- implType =%s, port =%d", implType, port)
	if cb == nil || port == 0 {
		return errors.New("CreateWebSocketServer args fail ！")
	}

	return self.initAndRun(implType, mode, port, cb)
}
