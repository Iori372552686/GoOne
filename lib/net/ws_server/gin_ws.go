package ws_server

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strconv"
)

var router *gin.Engine

// load  router
func (self *WsTcpSvr) loadRoutes() {
	router.GET("/ws", self.wsGinPageUpgrader)
}

// Run gin start the websocket server
func (self *WsTcpSvr) RunGinWs(mode string, wsPort int) error {
	port := strconv.Itoa(wsPort)
	if port == "" {
		return fmt.Errorf("port args err!")
	}

	if mode == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router = gin.Default()
	self.loadRoutes()

	go router.Run(":" + port)
	logger.Infof("------ Http Gin WsServer Running by :%v ------", port)
	return nil
}

// WsPage is gin websocket handler
func (self *WsTcpSvr) wsGinPageUpgrader(c *gin.Context) {
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}

	chanWrite := make(chan []byte, 100)
	self.lockOfConnInfo.Lock()
	self.mapOfConnInfo[socket.NetConn()] = chanWrite
	self.lockOfConnInfo.Unlock()

	//opt
	socket.NetConn().(*net.TCPConn).SetNoDelay(true) // true 表示禁用 Nagle
	go self.runConnRead(socket)
	go self.runConnWrite(socket, chanWrite)
	logger.Infof("gin webSocket 建立连接:%v", socket.RemoteAddr().String())
}
