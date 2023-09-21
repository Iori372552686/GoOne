package ws_gin

import (
	"GoOne/lib/api/logger"
	"GoOne/lib/net/ws_server"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

var router *gin.Engine

// load  router
func loadRoutes() {
	router.GET("/ws", ws_server.WsGinPage)
}

// Run gin start the websocket server
func RunGinWs(mode string, wsPort int) error {
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
	loadRoutes()

	go router.Run(":" + port)
	logger.Infof("------ Http Gin WsServer Running by :%v ------", port)
	return nil
}
