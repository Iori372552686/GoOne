
package web

import (
	"fmt"
	"net/http"
	"time"

	`GoOne/lib/web/models`

	"github.com/gorilla/websocket"
)

const (
	DefaultAppId = 101 // 默认平台Id
)


var (
	clientManager = NewClientManager()
	appIds        = []uint32{DefaultAppId, 102, 103, 104} // 全部的平台
	serverIp   string
	serverPort string
)

func NewClientMgr()  *ClientManager {
	return clientManager
}

func GetAppIds() []uint32 {
	return appIds
}


func InAppIds(appId uint32) (inAppId bool) {
	return
}

func GetServer() (server *models.Server) {
	server = models.NewServer(serverIp, serverPort)

	return
}

func IsLocal(server *models.Server) (isLocal bool) {
	if server.Ip == serverIp && server.Port == serverPort {
		isLocal = true
	}

	return
}


func WsInitClientPage(w http.ResponseWriter, req *http.Request) {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		fmt.Println("升级协议", "ua:", r.Header["User-Agent"], "referer:", r.Header["Referer"])

		return true
	}}).Upgrade(w, req, nil)
	if err != nil {
		http.NotFound(w, req)

		return
	}

	fmt.Println("webSocket 建立连接:", conn.RemoteAddr().String())

	currentTime := uint64(time.Now().Unix())
	client := NewClient(conn.RemoteAddr().String(), conn, currentTime)

	go client.read()
	go client.write()

	// 用户连接事件
	clientManager.Register <- client
}


func InitClientManager() {
	go clientManager.start()
}