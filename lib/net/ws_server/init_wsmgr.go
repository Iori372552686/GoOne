package ws_server

import (
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	DefaultAppId = 101 // 默认平台Id
)

var (
	clientManager = NewClientManager()
	appIds        = []uint32{DefaultAppId, 102, 103, 104} // 全部的平台
)

func NewWsClientMgr() *ClientManager {
	return clientManager
}

// WsBeegoPage is beego websocket handler
func WsBeegoPage(w http.ResponseWriter, req *http.Request) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		logger.Infof("升级协议 | ua:%v  ,referer:%v", r.Header["User-Agent"], r.Header["Referer"])
		return true
	}}).Upgrade(w, req, nil)
	if err != nil {
		http.NotFound(w, req)
		return
	}

	logger.Infof("beego webSocket 建立连接: %v", conn.RemoteAddr().String())
	currentTime := datetime.NowMs()
	client := NewClient(uint32(DefaultAppId), conn.RemoteAddr().String(), conn, currentTime)

	go client.read()
	go client.write()
	clientManager.Register <- client
}

func InitClientManager(cb func(client *Client, message []byte)) {
	clientManager.HandlerFunc = cb
	go clientManager.start()
}

// WsPage is gin websocket handler
func WsGinPage(c *gin.Context) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}

	//id, _ := uuid.NewV4()
	currentTime := datetime.NowMs()
	client := NewClient(uint32(DefaultAppId), conn.RemoteAddr().String(), conn, currentTime)
	clientManager.Register <- client
	logger.Infof("gin webSocket 建立连接:%v", conn.RemoteAddr().String())

	go client.read()
	go client.write()
}
