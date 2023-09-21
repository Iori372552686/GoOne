package net

import (
	"GoOne/lib/api/logger"
	gnet_svr "GoOne/lib/net/gnet_server"
	"GoOne/lib/net/net_mgr"
	"GoOne/lib/net/ws_server"
	ws_gin "GoOne/lib/net/ws_server/gin"
	"errors"
	"net"

	"github.com/panjf2000/gnet"
	Kcp "github.com/xtaci/kcp-go/v5"
)

// tcp impl
func CreateTcpServer(TcpSvr *net_mgr.ConnTcpSvr, implType string, port int, cb func(conn net.Conn, data []byte)) error {
	logger.Infof(" -----  CreateTcpServer ---- implType =%s, port =%d", implType, port)
	if cb == nil || TcpSvr == nil || port == 0 {
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
		return TcpSvr.InitAndRun("0.0.0.0", port, cb)
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
func CreateKcpServer(KcpSvr *net_mgr.ConnKcpSvr, port int, cb func(conn *Kcp.UDPSession, data []byte)) error {
	logger.Infof(" -----  CreateKcpServer ----, port =%d", port)
	if port == 0 || KcpSvr == nil || cb == nil {
		return errors.New("CreateKcpServer error, Args fail ！")
	}

	err := KcpSvr.InitAndRun(port, cb)
	if err != nil {
		logger.Errorf("CreateKcpServer InitAndRun ** fail ** !")
		return err
	}

	return nil
}

// websocket impl
func CreateWebSocketServer(implType, mode string, port int, cb func(client *ws_server.Client, message []byte)) error {
	logger.Infof(" -----  CreateWebSocketServer ---- implType =%s, port =%d", implType, port)
	if cb == nil || port == 0 {
		return errors.New("CreateWebSocketServer args fail ！")
	}

	ws_server.InitClientManager(cb)
	switch implType {
	case "gonet":
		//todo   -- need you, wait!
		return nil

	case "beego":
		//todo   -- need you, wait!
		return nil

	default: //"gin"
		return ws_gin.RunGinWs(mode, port)
	}

	return nil
}
