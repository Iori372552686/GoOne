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
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// webRuntimeComponent 把 websvr 的依赖初始化、HTTP/gRPC 启停包成单个 Component。
// 它实现 Drainer：HTTP 用 Shutdown、gRPC 用 GracefulStop；超时后强制 Stop。
//
// 关键修复：
//   - HTTP 与 gRPC 共享同一个已 Seal 的 Dispatcher（BuildWebDispatcher 只调用一次）。
//   - Drain 超时时保留 server 指针，由 Stop 强制 Close（旧实现超时即置 nil，丢失强关路径）。
//   - 实现 RuntimeErrorSource：HTTP/gRPC Serve 异常退出送入 channel，供 App 监督。
type webRuntimeComponent struct {
	mu      sync.RWMutex
	httpSrv *http.Server
	grpcSrv *grpc.Server
	// healthSrv 是 gRPC health 服务，Drain 时置 NOT_SERVING 使负载均衡摘流。
	healthSrv *health.Server

	// runtimeErrCh 汇聚 HTTP/gRPC Serve 的非预期退出错误（容量 1）。
	runtimeErrCh chan error
}

// Name 实现 runtime.Component。
func (w *webRuntimeComponent) Name() string { return "web_runtime" }

// RuntimeErrors 实现 RuntimeErrorSource。
func (w *webRuntimeComponent) RuntimeErrors() <-chan error {
	return w.runtimeErrCh
}

// reportRuntimeErr 把非预期 Serve 错误送入 channel（容量 1，不阻塞）。
func (w *webRuntimeComponent) reportRuntimeErr(err error) {
	if err == nil {
		return
	}
	select {
	case w.runtimeErrCh <- err:
	default:
	}
}

// Start 实现 runtime.Component：初始化依赖 + 启动 HTTP/gRPC。Start 失败时自行清理。
func (w *webRuntimeComponent) Start(_ context.Context) error {
	if w.runtimeErrCh == nil {
		w.runtimeErrCh = make(chan error, 1)
	}
	if err := globals.RedisMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.DbInstances); err != nil {
		return err
	}
	globals.SignMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.HTTPSigns)
	globals.RestMgr.Init(gconf.WebSvrCfg.Dependencies.RestApiConf, globals.SignMgr)
	sensitive_words.Init(gconf.WebSvrCfg.Dependencies.SensitiveWordsFile)

	// 构建一次 Dispatcher，HTTP 与 gRPC 共享。
	d, srv := controller.BuildWebDispatcher()

	httpSrv, httpServeErr, err := web_gin.StartGin(gconf.WebSvrCfg.Runtime.HttpServer, func(router *gin.Engine) {
		controller.LoadWebRoutesWithDispatcher(router, d, srv)
	})
	if err != nil {
		return err
	}
	w.setHTTPServer(httpSrv)
	// 监督 HTTP Serve 的非预期退出，送入 RuntimeErrorSource channel，
	// 使 HTTP listener 异常能触发 App Drain/Failed（与 gRPC 路径一致）。
	go func() {
		if err := <-httpServeErr; err != nil {
			w.reportRuntimeErr(fmt.Errorf("http: %w", err))
		}
	}()

	if err := w.startGRPCServer(d); err != nil {
		// 回滚已起的 HTTP。
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.shutdown(ctx)
		return err
	}
	return nil
}

// Drain 实现 runtime.Drainer：graceful 停 HTTP（Shutdown）与 gRPC（GracefulStop）。
// 超时时保留 server 指针，由 Stop 强制 Close（不在此处置 nil）。
func (w *webRuntimeComponent) Drain(ctx context.Context) error {
	return w.shutdown(ctx)
}

// Stop 实现 runtime.Component：强制关闭残留连接。
// 无论 Drain 是否成功/超时，Stop 必须对仍存在的 server 调用 Close 强制关闭。
// 同时关闭 Redis 连接池，消除连接泄漏。
func (w *webRuntimeComponent) Stop(_ context.Context) error {
	logger.Infof("================== websvr Stop =========================")
	var errs []error
	if grpcSrv := w.getGRPCServer(); grpcSrv != nil {
		grpcSrv.Stop()
		w.setGRPCServer(nil)
	}
	if httpSrv := w.getHTTPServer(); httpSrv != nil {
		if err := httpSrv.Close(); err != nil {
			errs = append(errs, err)
		}
		w.setHTTPServer(nil)
	}
	// 关闭 Redis 连接池，聚合 Close error。
	if err := globals.RedisMgr.Close(); err != nil {
		errs = append(errs, err)
	}
	// 停止远端配置热更并释放 config client（未启用远端时为空操作）。
	gamedata.StopNet()
	return errors.Join(errs...)
}

func (w *webRuntimeComponent) startGRPCServer(d *ssrpc.Dispatcher) error {
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

	srv := grpc.NewServer()
	d.MountGRPC(srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus("web.websvr.v1.WebApiService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	w.setHealthSrv(healthSrv)
	// reflection 仅在显式 debug 配置下启用，避免生产环境暴露服务元数据。
	if conf.Reflection {
		reflection.Register(srv)
	}
	w.setGRPCServer(srv)

	go func() {
		logger.Infof("------ gRPC Server Running by %v ------", addr)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Errorf("gRPC Serve error !! err=%v", err)
			w.reportRuntimeErr(err)
		}
	}()
	return nil
}

// shutdown 是 Drain 的实现。超时时保留 server 指针（不置 nil），让 Stop 能强关。
func (w *webRuntimeComponent) shutdown(ctx context.Context) error {
	var shutdownErr error
	// 先把 gRPC health 置 NOT_SERVING，使外部负载均衡/探针立即摘流，
	// 再 GracefulStop/Shutdown 等待在途请求。
	if hs := w.getHealthSrv(); hs != nil {
		hs.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		hs.SetServingStatus("web.websvr.v1.WebApiService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}
	grpcSrv := w.getGRPCServer()
	if grpcSrv != nil {
		done := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
			// GracefulStop 成功完成，清空指针。
			w.setGRPCServer(nil)
		case <-ctx.Done():
			// 超时：保留 grpcSrv 指针供 Stop 强制 Stop。这里不置 nil。
			if shutdownErr == nil {
				shutdownErr = ctx.Err()
			}
		}
	}
	httpSrv := w.getHTTPServer()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("http shutdown error | %v", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
			// 超时/失败：保留 httpSrv 指针供 Stop 强制 Close。
		} else {
			// Shutdown 成功，清空指针。
			w.setHTTPServer(nil)
		}
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

func (w *webRuntimeComponent) setHealthSrv(srv *health.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.healthSrv = srv
}

func (w *webRuntimeComponent) getHealthSrv() *health.Server {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.healthSrv
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

	app := runtime.MustNew("websvr",
		runtime.WithLoadConfig(func(_ context.Context) error {
			if err := gconf.LoadWebConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf loaded for websvr")
			// 配置中心（nacos）优先；未配置时回退本地目录。
			if gconf.WebSvrCfg.Dependencies.NacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", gconf.WebSvrCfg.Dependencies.NacosConf.GroupName)
				if err := gamedata.InitNacos(gconf.WebSvrCfg.Dependencies.NacosConf); err != nil {
					return err
				}
			} else if gconf.WebSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.WebSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.WebSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			return nil
		}),
	)

	// admin 在 LoadConfig 后用 WithAdminConfig 延迟读取端口/IP，复用
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

	// Start 顺序：datetime 周期刷新 → logger → admin → tracing → web 运行时
	//（依赖 + HTTP/gRPC）。admin 紧跟 logger，反向 Stop 时在 web 资源之后、logger 之前
	// 关闭。
	// 用 MustRegister 一次注册全部组件。
	app.MustRegister(scheduler.DefaultDateTimeTick(), logComp, adminComp, tracing, web)
	return app
}
