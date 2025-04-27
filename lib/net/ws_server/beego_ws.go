package ws_server

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

// WsBeegoPage is beego websocket handler
func (self *WsTcpSvr) WsBeegoPageUpgrader(w http.ResponseWriter, req *http.Request) {
	socket, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		logger.Infof("升级协议 | ua:%v  ,referer:%v", r.Header["User-Agent"], r.Header["Referer"])
		return true
	}}).Upgrade(w, req, nil)
	if err != nil {
		http.NotFound(w, req)
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
	logger.Infof("beego webSocket 建立连接:%v", socket.RemoteAddr().String())
}
