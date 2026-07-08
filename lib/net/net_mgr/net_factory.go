package net_mgr

import (
	"errors"
	"fmt"
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	gnet_svr "github.com/Iori372552686/GoOne/lib/net/gnet_server"
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
	case "gev", "gnet":
		// 未实现的事件驱动后端：显式报错，避免调用方误以为服务已启动。
		return fmt.Errorf("tcp implType %q is not implemented yet, use default (gonet)", implType)

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
	case "gev", "gonet":
		// 未实现的后端：显式报错，避免调用方误以为服务已启动。
		return fmt.Errorf("udp implType %q is not implemented yet, use default (gnet)", implType)

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
