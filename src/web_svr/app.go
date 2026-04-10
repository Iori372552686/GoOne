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
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/web_svr/controller"
	"github.com/Iori372552686/GoOne/src/web_svr/globals"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type webRuntime struct {
	mu      sync.RWMutex
	httpSrv *http.Server
	grpcSrv *grpc.Server
}

func NewApp() *bootstrap.ServiceApp {
	runtime := &webRuntime{}
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: "websvr",
		LoadConfig: func() error {
			if err := gconf.LoadWebConfig(*gconf.SvrConfFile); err != nil {
				return err
			}
			logger.Infof("svr_conf: %+v", gconf.WebSvrCfg)
			if gconf.WebSvrCfg.Dependencies.GameDataDir != "" {
				logger.Infof("Loading local file by gameconf_dir: %v ", gconf.WebSvrCfg.Dependencies.GameDataDir)
				if err := gamedata.InitLocal(gconf.WebSvrCfg.Dependencies.GameDataDir); err != nil {
					return err
				}
			}
			return nil
		},
		LoggerConfig: func() bootstrap.LoggerConfig {
			return bootstrap.LoggerConfig{
				Dir:   gconf.WebSvrCfg.Debug.LogDir,
				Level: gconf.WebSvrCfg.Debug.LogLevel,
				Name:  "websvr",
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			return bootstrap.NewAdminConfig(
				"websvr",
				misc.ServerType_WebSvr,
				gconf.WebSvrCfg.CommonRuntime.AdminServer.Enabled,
				gconf.WebSvrCfg.CommonDebug.Pprof,
				gconf.WebSvrCfg.CommonRuntime.AdminServer.IP,
				gconf.WebSvrCfg.CommonRuntime.AdminServer.Port,
			)
		},
		ComponentStatuses: func() []bootstrap.ComponentStatus {
			return runtime.componentStatuses()
		},
		InitDeps: func() error {
			if err := ssrpc.InitTracing("websvr", ssrpc.TracingConfig{
				Enabled:      gconf.WebSvrCfg.CommonRuntime.Tracing.Enabled,
				Exporter:     gconf.WebSvrCfg.CommonRuntime.Tracing.Exporter,
				Endpoint:     gconf.WebSvrCfg.CommonRuntime.Tracing.Endpoint,
				Insecure:     gconf.WebSvrCfg.CommonRuntime.Tracing.Insecure,
				SamplerRatio: gconf.WebSvrCfg.CommonRuntime.Tracing.SamplerRatio,
				Headers:      gconf.WebSvrCfg.CommonRuntime.Tracing.Headers,
			}); err != nil {
				return err
			}
			if err := globals.RedisMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.DbInstances); err != nil {
				return err
			}
			globals.SignMgr.InitAndRun(gconf.WebSvrCfg.Dependencies.HTTPSigns)
			globals.RestMgr.Init(gconf.WebSvrCfg.Dependencies.RestApiConf, globals.SignMgr)
			sensitive_words.Init(gconf.WebSvrCfg.Dependencies.SensitiveWordsFile)
			return nil
		},
		StartRuntime: func() error {
			httpSrv, err := web_gin.StartGin(gconf.WebSvrCfg.Runtime.HttpServer, controller.LoadWebRoutes)
			if err != nil {
				return err
			}
			runtime.setHTTPServer(httpSrv)
			if err := runtime.startGRPCServer(); err != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = runtime.shutdown(ctx)
				return err
			}
			return nil
		},
		OnProc: func() bool {
			return true
		},
		OnShutdown: func(ctx context.Context) error {
			shutdownErr := runtime.shutdown(ctx)
			if err := ssrpc.ShutdownTracing(ctx); err != nil {
				shutdownErr = errors.Join(shutdownErr, err)
			}
			return shutdownErr
		},
		OnExit: func() {
			logger.Infof("================== websvr Stop =========================")
		},
	})
}

func (r *webRuntime) startGRPCServer() error {
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
	r.setGRPCServer(srv)

	go func() {
		logger.Infof("------ gRPC Server Running by %v ------", addr)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Errorf("gRPC Serve error !! err=%v", err)
		}
	}()
	return nil
}

func (r *webRuntime) shutdown(ctx context.Context) error {
	var shutdownErr error
	grpcSrv := r.getGRPCServer()
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
		r.setGRPCServer(nil)
	}
	httpSrv := r.getHTTPServer()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("http shutdown error | %v", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
		r.setHTTPServer(nil)
	}
	return shutdownErr
}

func (r *webRuntime) setHTTPServer(srv *http.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.httpSrv = srv
}

func (r *webRuntime) getHTTPServer() *http.Server {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.httpSrv
}

func (r *webRuntime) setGRPCServer(srv *grpc.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grpcSrv = srv
}

func (r *webRuntime) getGRPCServer() *grpc.Server {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.grpcSrv
}

func (r *webRuntime) componentStatuses() []bootstrap.ComponentStatus {
	return buildWebComponentStatuses(
		globals.RedisMgr.InstanceCount(),
		globals.SignMgr.Count(),
		globals.RestMgr.Count(),
		r.getHTTPServer(),
		gconf.WebSvrCfg.Runtime.HttpServer,
		r.getGRPCServer(),
		gconf.WebSvrCfg.Runtime.GRPCServer,
	)
}

func buildWebComponentStatuses(redisInstances, signInstances, restInstances int, httpSrv *http.Server, httpCfg web_gin.Config, grpcSrv *grpc.Server, grpcCfg gconf.GRPCServerConfig) []bootstrap.ComponentStatus {
	httpStatus := bootstrap.ComponentStatus{
		Name:    "websvr.http_server",
		State:   "pending",
		Ready:   false,
		Message: fmt.Sprintf("configured addr=%s:%d", httpCfg.IP, httpCfg.Port),
	}
	if httpSrv != nil {
		httpStatus.State = "ready"
		httpStatus.Ready = true
		httpStatus.Message = fmt.Sprintf("listening on %s", httpSrv.Addr)
	}

	grpcStatus := bootstrap.ComponentStatus{
		Name:    "websvr.grpc_server",
		State:   "skipped",
		Ready:   true,
		Message: "grpc disabled",
	}
	if grpcCfg.Enabled {
		grpcStatus.State = "pending"
		grpcStatus.Ready = false
		grpcStatus.Message = fmt.Sprintf("configured addr=%s:%d", grpcCfg.IP, grpcCfg.Port)
		if grpcSrv != nil {
			grpcStatus.State = "ready"
			grpcStatus.Ready = true
			grpcStatus.Message = fmt.Sprintf("listening on %s:%d", grpcCfg.IP, grpcCfg.Port)
		}
	}

	redisStatus := bootstrap.ComponentStatus{
		Name:    "websvr.redis",
		State:   "pending",
		Ready:   false,
		Message: "waiting for redis initialization",
	}
	if redisInstances > 0 {
		redisStatus.State = "ready"
		redisStatus.Ready = true
		redisStatus.Message = fmt.Sprintf("redis instances=%d", redisInstances)
	}

	signStatus := bootstrap.ComponentStatus{
		Name:    "websvr.http_sign",
		State:   "pending",
		Ready:   false,
		Message: "waiting for sign initialization",
	}
	if signInstances > 0 {
		signStatus.State = "ready"
		signStatus.Ready = true
		signStatus.Message = fmt.Sprintf("sign instances=%d", signInstances)
	}

	restStatus := bootstrap.ComponentStatus{
		Name:    "websvr.rest_api",
		State:   "pending",
		Ready:   false,
		Message: "waiting for rest api initialization",
	}
	if restInstances > 0 {
		restStatus.State = "ready"
		restStatus.Ready = true
		restStatus.Message = fmt.Sprintf("rest api instances=%d", restInstances)
	}

	return []bootstrap.ComponentStatus{
		redisStatus,
		signStatus,
		restStatus,
		httpStatus,
		grpcStatus,
	}
}
