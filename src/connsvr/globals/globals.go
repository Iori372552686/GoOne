package globals

import (
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
)

var (
	TransMgr               = transaction.NewTransactionMgr()
	// SessionTracker 跟踪网关活跃连接/会话。SessionHub 把它注入三传输，供
	// gatewayComponent.Drain 等待逻辑会话归零。
	SessionTracker         = runtime.NewSessionTracker()
	// SessionHub 是三传输共享的会话状态拥有者。同一 UID 跨 TCP/WS/KCP 重绑
	// 原子化；Drain 只等 ActiveSessions。
	SessionHub             = net_mgr.NewSessionHub(SessionTracker)
	ConnTcpSvr             = newTcpSvrWithHub()
	ConnWsSvr              = newWsSvrWithHub()
	ConnKcpSvr             = newKcpSvrWithHub()
	SignMgr                = http_sign.NewSignMgr()
	RestMgr                = rest_api.NewRestApiMgr()
	ClientPacketDispatcher = ssrpc.NewDispatcher()
)

// newTcpSvrWithHub 创建 TCP 网关并注入共享 SessionHub。
func newTcpSvrWithHub() *net_mgr.ConnTcpSvr {
	s := net_mgr.NewTcpSvr()
	s.SetHub(SessionHub)
	return s
}

func newWsSvrWithHub() *net_mgr.ConnWsTcpSvr {
	s := net_mgr.NewWsTcpSvr()
	s.SetHub(SessionHub)
	return s
}

func newKcpSvrWithHub() *net_mgr.ConnKcpSvr {
	s := net_mgr.NewKcpSvr()
	s.SetHub(SessionHub)
	return s
}
