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
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
	"github.com/Iori372552686/GoOne/module/conf"
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
	if err := globals.RedisMgr.OnStart(nil); err != nil {
		return err
	}
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
	sensitive_words.Init(conf.Get("base_cfg.dependencies.sensitive_words_file").String())

	// 构建一次 Dispatcher，HTTP 与 gRPC 共享。
	d, srv := controller.BuildWebDispatcher()

	var httpCfg web_gin.Config
	if err := conf.Unmarshal("websvr.runtime.http_server", &httpCfg); err != nil {
		return err
	}
	httpSrv, httpServeErr, err := web_gin.StartGin(httpCfg, func(router *gin.Engine) {
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
	var grpcCfg gconf.GRPCServerConfig
	if err := conf.Unmarshal("websvr.runtime.grpc_server", &grpcCfg); err != nil {
		return err
	}
	if !grpcCfg.Enabled {
		return nil
	}
	if grpcCfg.Port <= 0 {
		return errors.New("grpc_server.port args err!")
	}

	addr := fmt.Sprintf("%s:%d", grpcCfg.IP, grpcCfg.Port)
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
	if grpcCfg.Reflection {
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
// 服务名只在 MustNew 出现一次；标准组件（logger/admin/tracing）由 bussvc 构造器
// 自读 conf 装配。websvr 无 bus/router，admin 不接运行期就绪探针。
func NewApp() *runtime.App {
	// gamedata 加载作为 LoadConfig 的追加钩子：配置中心（nacos）优先，
	// 未配置时回退本地目录。
	app := bussvc.MustNew("websvr", nil, bussvc.WithConfLoader(func(_ context.Context) error {
		var nacosConf net_conf.NacosConf
		_ = conf.Unmarshal("base_cfg.dependencies.nacos_conf", &nacosConf)
		gameDataDir := conf.Get("base_cfg.dependencies.game_data_dir").String()
		if nacosConf.IPAddr != "" {
			logger.Infof("Loading remote gameconf by Nacos group: %v ", nacosConf.GroupName)
			if err := gamedata.InitNacos(nacosConf); err != nil {
				return err
			}
		} else if gameDataDir != "" {
			logger.Infof("Loading local file by gameconf_dir: %v ", gameDataDir)
			if err := gamedata.InitLocal(gameDataDir); err != nil {
				return err
			}
		}
		return nil
	}))

	// 标准组件（datetime/logger/admin/tracing）由 bussvc.MustNew 集中注册。
	web := &webRuntimeComponent{}
	app.MustRegister(web)
	return app
}
