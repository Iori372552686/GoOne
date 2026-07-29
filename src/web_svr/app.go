package websvr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/common/gamedata"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/scheduler"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
	"github.com/Iori372552686/GoOne/module/gconf"
	"github.com/Iori372552686/GoOne/src/web_svr/controller"
	"github.com/Iori372552686/GoOne/src/web_svr/globals"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// webRuntimeComponent 把 websvr 的依赖初始化、HTTP/gRPC 启停包成单个 Component。
// 它实现 Drainer：HTTP 用 Shutdown、gRPC 用 GracefulStop；超时后强制 Stop。
type webRuntimeComponent struct {
	mu      sync.RWMutex
	httpSrv *http.Server
	grpcSrv *grpc.Server
}

// Name 实现 runtime.Component。
func (w *webRuntimeComponent) Name() string { return "web_runtime" }

// Start 实现 runtime.Component：初始化依赖 + 启动 HTTP/gRPC。Start 失败时自行清理。
func (w *webRuntimeComponent) Start(_ context.Context) error {
	if err := globals.RedisMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.DbInstances); err != nil {
		return err
	}
	globals.SignMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.HTTPSigns)
	globals.RestMgr.Init(gconf.WebSvrCfg.Dependencies.RestApiConf, globals.SignMgr)
	sensitive_words.Init(gconf.WebSvrCfg.Dependencies.SensitiveWordsFile)

	httpSrv, err := web_gin.StartGin(gconf.WebSvrCfg.Runtime.HttpServer, controller.LoadWebRoutes)
	if err != nil {
		return err
	}
	w.setHTTPServer(httpSrv)
	if err := w.startGRPCServer(); err != nil {
		// 回滚已起的 HTTP。
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.shutdown(ctx)
		return err
	}
	return nil
}

// Drain 实现 runtime.Drainer：graceful 停 HTTP（Shutdown）与 gRPC（GracefulStop）。
func (w *webRuntimeComponent) Drain(ctx context.Context) error {
	return w.shutdown(ctx)
}

// Stop 实现 runtime.Component：强制关闭残留连接。
func (w *webRuntimeComponent) Stop(_ context.Context) error {
	logger.Infof("================== websvr Stop =========================")
	return nil
}

func (w *webRuntimeComponent) startGRPCServer() error {
	conf := gconf.WebSvrCfg.Runtime.GRPCServer
	if !conf.Enabled {
		return nil
	}
	if conf.Port <= 0 {
		return errors.New("grpc_server.port args err!")
	}

	addr := fmt.Sprintf("%s:%d", conf.IP, conf.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	d, _ := controller.BuildWebDispatcher()
	srv := grpc.NewServer()
	d.MountGRPC(srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus("web.websvr.v1.WebApiService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)
	w.setGRPCServer(srv)

	go func() {
		logger.Infof("------ gRPC Server Running by %v ------", addr)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Errorf("gRPC Serve error !! err=%v", err)
		}
	}()
	return nil
}

func (w *webRuntimeComponent) shutdown(ctx context.Context) error {
	var shutdownErr error
	grpcSrv := w.getGRPCServer()
	if grpcSrv != nil {
		done := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			grpcSrv.Stop()
			if shutdownErr == nil {
				shutdownErr = ctx.Err()
			}
		}
		w.setGRPCServer(nil)
	}
	httpSrv := w.getHTTPServer()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("http shutdown error | %v", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
		w.setHTTPServer(nil)
	}
	return shutdownErr
}

func (w *webRuntimeComponent) setHTTPServer(srv *http.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.httpSrv = srv
}

func (w *webRuntimeComponent) getHTTPServer() *http.Server {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.httpSrv
}

func (w *webRuntimeComponent) setGRPCServer(srv *grpc.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.grpcSrv = srv
}

func (w *webRuntimeComponent) getGRPCServer() *grpc.Server {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.grpcSrv
}

// NewApp 用 runtime.App + Component 装配 websvr（非 bus 服务：HTTP + 可选 gRPC）。
func NewApp() *runtime.App {
	web := &webRuntimeComponent{}

	tracing := &bussvc.TracingComponent{
		ServiceName: "websvr",
		Cfg: func() ssrpc.TracingConfig {
			t := gconf.WebSvrCfg.CommonRuntime.Tracing
			return ssrpc.TracingConfig{
				Enabled:      t.Enabled,
				Exporter:     t.Exporter,
				Endpoint:     t.Endpoint,
				Insecure:     t.Insecure,
				SamplerRatio: t.SamplerRatio,
				Headers:      t.Headers,
			}
		},
	}
	logComp := &bussvc.LoggerComponent{
		Cfg: func() bussvc.LoggerConfig {
			return bussvc.LoggerConfig{
				Dir:   gconf.WebSvrCfg.Debug.LogDir,
				Level: gconf.WebSvrCfg.Debug.LogLevel,
				Name:  "websvr",
			}
		},
	}

	app, err := runtime.New("websvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadWebConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for websvr")
			if gconf.WebSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.WebSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.WebSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("runtime.New websvr: %v", err))
	}

	// P0-03：admin 在 LoadConfig 后用 WithAdminConfig 延迟读取端口/IP，复用
	// app.tracker。
	adminComp := runtime.NewAdminComponent(app,
		runtime.WithAdminConfig(func() runtime.AdminConfig {
			wc := gconf.WebSvrCfg.CommonRuntime
			return runtime.AdminConfig{
				Enabled: wc.AdminServer.Enabled,
				IP:      wc.AdminServer.IP,
				Port:    wc.AdminServer.Port,
				Pprof:   gconf.WebSvrCfg.CommonDebug.Pprof,
			}
		}),
		runtime.WithAdminServiceName("websvr"),
	)

	// Start 顺序（P0-03）：datetime 周期刷新 → logger → admin → tracing → web 运行时
	//（依赖 + HTTP/gRPC）。admin 紧跟 logger，反向 Stop 时在 web 资源之后、logger 之前
	// 关闭。
	for _, c := range []runtime.Component{scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing, web} {
		if err := app.Register(c); err != nil {
			panic(fmt.Sprintf("websvr register %s: %v", c.Name(), err))
		}
	}
	return app
}
