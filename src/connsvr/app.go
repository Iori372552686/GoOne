package connsvr

import (
	"context"
	"fmt"

	connsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/connsvr/v1"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/service"
)

// NewApp 用 runtime.App + Component 装配 connsvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
func NewApp() *runtime.App {
	app := bussvc.MustNew("connsvr", router.ReadyCheck, bussvc.WithConfLoader())

	// 标准组件（datetime/logger/admin/tracing）由 bussvc.MustNew 集中注册。

	transMgr := &bussvc.TransMgrComponent{Mgr: globals.TransMgr}

	signRestDeps := &bussvc.FuncComponent{
		ComponentName: "sign_rest_deps",
		OnStart: func(_ context.Context) error {
			var signs []http_sign.Config
			if err := conf.Unmarshal("base_cfg.dependencies.http_sign", &signs); err != nil {
				return err
			}
			globals.SignMgr.InitAndRun(signs)
			var restConf []rest_api.Config
			if err := conf.Unmarshal("base_cfg.dependencies.rest_api_config", &restConf); err != nil {
				return err
			}
			globals.RestMgr.Init(restConf, globals.SignMgr)
			return nil
		},
	}

	// 用 RegistryComponent 替代 "NewDispatcher→ToDispatcher→丢弃" 闭包。
	// Register→Seal→TransMgr 绑定在一个组件内原子完成。
	registerHandlers := ssrpc.NewRegistryComponent(
		"ssrpc_registry",
		func(r *ssrpc.Registry) error {
			srv := connsvrv1.NewConnServiceSServer(&service.ConnServiceImpl{}, ssrpc.DefaultMWOptions{})
			return connsvrv1.RegisterConnServiceToRegistry(r, srv)
		},
		ssrpc.WithTransactionManager(globals.TransMgr),
	)

	// connsvr 特有：客户端下行包在 onRecvSSPacket 中短路回客户端，覆盖默认的
	// TransMgr.ProcessSSPacket。DriverRegistry 只注册 rabbitmq（bus 服务不再隐式
	// 链接全部 5 类 MQ SDK；websvr 不链接任何 driver）。
	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry(), onRecvSSPacket)

	// 网关监听器：在 router/bus 起来之后启动 TCP/WS/KCP。实现 Quiescer（停止接新连接）
	// 与 runtime.Component（Stop 强制关闭全部连接），满足网关排空。
	gateway := &gatewayComponent{}

	// 用 MustRegister 一次注册全部组件（顺序即 Start 顺序）。
	app.MustRegister(
		signRestDeps, registerHandlers, transMgr, routerComp, gateway,
	)
	return app
}

// gatewayComponent 管理三传输（TCP/WS/KCP）网关监听器的启动、排空与停止。
// Start 拉起监听；Quiesce 停止接新连接但保留既有；Stop 强制关闭全部连接。
type gatewayComponent struct{}

func (gatewayComponent) Name() string { return "gateway_listeners" }

// Start 启动 TCP/WS/KCP 监听器。任一传输启动失败时，逆序 Stop 已成功启动的传输，
// 合并原始启动错误与回滚错误，并在错误中标注传输名（tcp/ws/kcp），避免端口泄漏
// 。
func (gatewayComponent) Start(_ context.Context) error {
	// 用已加载的 capacity 配置构造过载保护闸门，注入共享 SessionHub。
	// 三传输 OnConn 与 pack_proc 首次登录通过 hub 间接调用闸门。off 模式直通。
	// capacity 段可选：缺省时零值即 OverloadModeOff（不限制），向后兼容。
	var cap gconf.ConnCapacityConfig
	if conf.Has("connsvr.capacity") {
		if err := conf.Unmarshal("connsvr.capacity", &cap); err != nil {
			return err
		}
	}
	globals.SessionHub.SetAdmission(net_mgr.NewAdmissionController(globals.SessionHub, net_mgr.AdmissionLimits{
		MaxConnections:                cap.MaxConnections,
		MaxUnauthenticatedConnections: cap.MaxUnauthenticatedConnections,
		ConnectionRate:                cap.ConnectionRate,
		LoginRate:                     cap.LoginRate,
		MaxInflight:                   cap.MaxInflight,
		OverloadMode:                  cap.OverloadMode,
	}))

	// 网关监听端口/TCP 后端类型/KCP 端口从 conf 读取。
	tcpImplType := conf.Get("connsvr.runtime.tcp_impl_type").String()
	listenPort := conf.Get("connsvr.runtime.listen_port").Int()
	kcpPort := conf.Get("connsvr.runtime.kcp_port").Int()

	// started 记录已成功启动的传输及其 Stop 函数，用于部分启动失败时逆序回滚。
	// Stop 签名为 Stop(context.Context) error：ctx 约束强制关闭、错误可观测。
	started := make([]struct {
		name string
		stop func(context.Context) error
	}, 0, 3)
	rollback := func(startErr error) error {
		// 逆序回滚已启动传输。Stop 幂等；回滚期用 context.Background() 不受超时约束，
		// 仅尽力关闭监听器避免端口泄漏。Stop 返回的错误被忽略：回滚的目的是清理，
		// 原始启动错误（已含传输名）作为返回值保留。
		ctx := context.Background()
		for i := len(started) - 1; i >= 0; i-- {
			_ = started[i].stop(ctx)
		}
		return startErr
	}

	if err := globals.ConnTcpSvr.CreateTcpServer(
		tcpImplType,
		listenPort+1, onTcpPacket); err != nil {
		return rollback(fmt.Errorf("tcp: %w", err))
	}
	started = append(started, struct {
		name string
		stop func(context.Context) error
	}{"tcp", globals.ConnTcpSvr.Stop})

	if err := globals.ConnWsSvr.CreateWebSocketServer(
		"gin", "debug", listenPort, onWebSocketPacket); err != nil {
		return rollback(fmt.Errorf("ws: %w", err))
	}
	started = append(started, struct {
		name string
		stop func(context.Context) error
	}{"ws", globals.ConnWsSvr.Stop})

	if kcpPort > 0 {
		if err := globals.ConnKcpSvr.CreateKcpServer(kcpPort, onKcpPacket); err != nil {
			return rollback(fmt.Errorf("kcp: %w", err))
		}
		started = append(started, struct {
			name string
			stop func(context.Context) error
		}{"kcp", globals.ConnKcpSvr.Stop})
	}
	return nil
}

// Quiesce 实现 runtime.Quiescer：三传输停止接收新连接，SessionHub 拒绝新会话绑定
// 。保留既有连接处理在途工作。readyz 此刻已返回 503（由状态机保证）。
func (gatewayComponent) Quiesce(_ context.Context) error {
	// 先让 hub 拒绝新 BindClient（draining），再停三传输 listener。
	globals.SessionHub.Quiesce()
	globals.ConnTcpSvr.Quiesce()
	globals.ConnWsSvr.Quiesce()
	globals.ConnKcpSvr.Quiesce()
	return nil
}

// Drain 实现 runtime.Drainer：等待逻辑会话（ActiveSessions）归零。
// 只等 session，不等未认证连接；超时由 App 的 drain 超时统一处理。
func (gatewayComponent) Drain(ctx context.Context) error {
	return globals.SessionTracker.WaitSessions(ctx)
}

// Stop 实现 runtime.Component：强制关闭三传输的全部残留连接，并关闭 SessionTracker
// （唤醒任何仍在等待的 Drain）。幂等。ctx 透传给三传输 Stop（受 context
// 约束、错误可观测）。
func (gatewayComponent) Stop(ctx context.Context) error {
	globals.ConnTcpSvr.Stop(ctx)
	globals.ConnWsSvr.Stop(ctx)
	globals.ConnKcpSvr.Stop(ctx)
	globals.SessionTracker.Close()
	return nil
}
