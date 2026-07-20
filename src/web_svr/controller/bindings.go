package controller

import (
	websvrv1 "github.com/Iori372552686/GoOne/api/gen/web/websvr/v1"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/src/web_svr/globals"
	"github.com/Iori372552686/GoOne/src/web_svr/service"
)

// BuildWebDispatcher wires the generated ssrpc bindings used by both HTTP and gRPC.
func BuildWebDispatcher() (*ssrpc.Dispatcher, websvrv1.WebApiServiceSServer) {
	srv := websvrv1.NewWebApiServiceSServer(&service.WebApiServiceImpl{}, ssrpc.DefaultMWOptions{
		Sign: service.NewHTTPSignVerifier(gconf.WebSvrCfg.Runtime.HttpServer.AuthEnable, globals.SignMgr.GetSignIns()),
	})
	d := ssrpc.NewDispatcher()
	websvrv1.RegisterWebApiServiceToDispatcher(d, srv)
	return d, srv
}
