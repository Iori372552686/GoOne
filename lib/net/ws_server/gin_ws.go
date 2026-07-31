package ws_server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
	"github.com/gin-gonic/gin"
)

// RunGinWs 同步绑定 HTTP 监听器并启动 Gin WebSocket 服务。
//
// 关键修复：
//   - 同步 net.Listen 使端口冲突在 Start 期（而非稍后的 goroutine 内）返回。
//   - 删除包级全局 gin.Engine，改为实例字段，支持同进程多 WS 实例。
//   - 保存 listener / http.Server，使 Quiesce 能关闭 listener 停止新 Upgrade、Stop 能
//     强制 Close 残留连接。
//   - Serve goroutine 的非预期退出错误送入 serveErr（容量 1），供上层监督（如
//     RuntimeErrorSource）感知监听器死亡；预期的 http.ErrServerClosed 不上报。
func (s *WsTcpSvr) RunGinWs(mode string, wsPort int) error {
	port := strconv.Itoa(wsPort)
	if port == "" {
		return fmt.Errorf("port args err!")
	}

	if mode == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 同步绑定，使端口冲突立即返回。
	addr := ":" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ws server listen %s: %w", addr, err)
	}
	s.httpListener = ln

	router := gin.Default()
	router.GET("/ws", s.wsGinPageUpgrader)

	srv := &http.Server{Handler: router}
	s.httpServer = srv
	if s.serveErr == nil {
		s.serveErr = make(chan error, 1)
	}

	go func() {
		logger.Infof("------ Http Gin WsServer Running on %s ------", port)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("ws gin server stopped with error | %v", err)
			select {
			case s.serveErr <- err:
			default:
			}
		}
	}()
	return nil
}

// wsGinPageUpgrader 是 gin websocket 升级 handler。
func (s *WsTcpSvr) wsGinPageUpgrader(c *gin.Context) {
	// Quiesce 后拒绝新 Upgrade。
	if !s.accepting.Load() {
		http.Error(c.Writer, "shutting down", http.StatusServiceUnavailable)
		return
	}
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}

	chanWrite := make(chan *bufpool.Buffer, 100)
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo[socket.NetConn()] = &wsConnEntry{chanWrite: chanWrite}
	s.lockOfConnInfo.Unlock()

	//opt
	if tcpConn, ok := socket.NetConn().(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true) // true 表示禁用 Nagle
	}

	s.handler.OnConn(socket.NetConn())
	go s.runConnRead(socket)
	go s.runConnWrite(socket, chanWrite)
	logger.Infof("gin webSocket 建立连接:%v", socket.RemoteAddr().String())
}

// ServeErrors 返回 ws gin server 的运行期错误 channel（容量 1）。非预期退出（非
// http.ErrServerClosed）会送入此 channel，供上层监督监听器死亡。
func (s *WsTcpSvr) ServeErrors() <-chan error {
	return s.serveErr
}
