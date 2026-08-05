package controller

import (
	websvrv1 "github.com/Iori372552686/GoOne/api/gen/web/websvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/web_svr/globals"
	"github.com/Iori372552686/GoOne/src/web_svr/service"
)

// BuildWebDispatcher wires the generated ssrpc bindings used by both HTTP and gRPC.
//
// 经 Registry 装配（RegisterWebApiServiceToRegistry）后 Seal，使 HTTP 与 gRPC 共
// 享同一个已 Seal 的 Dispatcher，并复用 Registry 的原子校验/查重。Seal 后的 Dispatcher
// 热路径无锁。
func BuildWebDispatcher() (*ssrpc.Dispatcher, websvrv1.WebApiServiceSServer) {
	srv := websvrv1.NewWebApiServiceSServer(&service.WebApiServiceImpl{}, ssrpc.DefaultMWOptions{
		Sign: service.NewHTTPSignVerifier(conf.Get("websvr.runtime.http_server.auth_enable").Bool(), globals.SignMgr.GetSignIns()),
	})
	r := ssrpc.NewRegistry()
	if err := websvrv1.RegisterWebApiServiceToRegistry(r, srv); err != nil {
		logger.Errorf("BuildWebDispatcher register: %v", err)
		return nil, srv
	}
	d, err := r.Seal()
	if err != nil {
		logger.Errorf("BuildWebDispatcher seal: %v", err)
		return nil, srv
	}
	return d, srv
}
