package controller

import (
	"github.com/gin-gonic/gin"
	websvrv1 "github.com/Iori372552686/GoOne/api/gen/web/websvr/v1"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
)

// load web router
func LoadWebRoutes(router *gin.Engine) {
	//globals.PromMgr.SetGinMidAndRouter(router) // add mid and router

	// Phase 2 (new routes): IDL-driven HTTP handlers (gin) -> ssrpc runtime -> service implementation.
	d, srv := BuildWebDispatcher()
	d.MountGin(router)

	// Phase 1 (legacy routes): direct gin handlers -> legacy service functions.
	RegisterLegacyWebRoutes(router, srv)
}

// LoadWebRoutesWithDispatcher 与 LoadWebRoutes 相同，但复用调用方已构建的 Dispatcher
// 与 SServer（P0-07：HTTP 与 gRPC 必须共享同一个已 Seal 的 Dispatcher，禁止各自
// BuildWebDispatcher 构建两次）。
func LoadWebRoutesWithDispatcher(router *gin.Engine, d *ssrpc.Dispatcher, srv websvrv1.WebApiServiceSServer) {
	d.MountGin(router)
	RegisterLegacyWebRoutes(router, srv)
}
