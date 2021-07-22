package routers

import (
	`GoOne/lib/web/controllers`

	"github.com/astaxie/beego"
)

func InitRouter() {
	// Register routers.
	beego.Router("/", &controllers.AppController{})
	// Indicate AppController.Join method to handle POST requests.
	beego.Router("/join", &controllers.AppController{}, "post:Join")

	// Long polling.
	beego.Router("/lp", &controllers.LongPollingController{}, "get:Join")
	beego.Router("/lp/post", &controllers.LongPollingController{})
	beego.Router("/lp/fetch", &controllers.LongPollingController{}, "get:Fetch")

	// WebSocket.
	beego.Router("/ws", &controllers.WebSocketController{})
	beego.Router("/ws/join", &controllers.WebSocketController{}, "get:Join")
	beego.Router("/ws/heartbeat", &controllers.WebSocketController{},"get:Heart")
	//beego.Router("/ws/ping", &controllers.WebSocketController{},"get:ping")

}
