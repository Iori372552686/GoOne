package net_mgr

import (
	"errors"
	"fmt"
	"net"

	"github.com/Iori372552686/GoOne/lib/api/logger"
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

// websocket impl
func (self *ConnWsTcpSvr) CreateWebSocketServer(implType, mode string, port int, cb func(conn net.Conn, data []byte)) error {
	logger.Infof(" -----  CreateWebSocketServer ---- implType =%s, port =%d", implType, port)
	if cb == nil || port == 0 {
		return errors.New("CreateWebSocketServer args fail ！")
	}

	return self.initAndRun(implType, mode, port, cb)
}
