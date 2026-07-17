package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AdminOption 配置 admin server。
type AdminOption func(*adminConfig)

type adminConfig struct {
	enabled     bool
	ip          string
	port        int
	serviceName string
	pprof       bool
}

// WithAdminListen 设置绑定地址与端口。空 IP 默认回环地址 127.0.0.1（绝不所有接
// 口），使 admin 面（含 pprof）不会意外对外暴露。
func WithAdminListen(ip string, port int) AdminOption {
	return func(c *adminConfig) {
		c.enabled = true
		c.ip = ip
		c.port = port
	}
}

// WithAdminPprof 在 admin server 上启用 pprof handler。要求 admin server 自身被启
// 用（WithAdminListen）且仅在其上生效；pprof 绝不挂载到公共监听器。
func WithAdminPprof(enable bool) AdminOption {
	return func(c *adminConfig) {
		c.pprof = enable
		if enable {
			c.enabled = true
		}
	}
}

// WithAdminServiceName 用服务名标注 admin 响应。
func WithAdminServiceName(name string) AdminOption {
	return func(c *adminConfig) { c.serviceName = name }
}

// AdminComponent 把一个 HTTP admin server 包装为 Component。它暴露
// /healthz、/readyz、/statez、/info、/components 与 /metrics，可选 pprof。它实现
// Drainer（HTTP Shutdown），使在途 admin 请求在排空期获得宽限窗口。
//
// Start 同步绑定监听器，使端口冲突在启动期而非稍后暴露。Stop 带超时关闭 server。
type AdminComponent struct {
	cfg     adminConfig
	app     *App
	state   *StateStore
	tracker *ComponentTracker
	srv     *http.Server
	ln      net.Listener
	addr    string
}

// NewAdminComponent 构建一个绑定到给定 App 的 state store 与 component tracker
// 的 admin Component。必须在 Run 之前构造。
func NewAdminComponent(app *App, tracker *ComponentTracker, opts ...AdminOption) *AdminComponent {
	cfg := adminConfig{serviceName: app.Name()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.ip == "" {
		cfg.ip = "127.0.0.1"
	}
	return &AdminComponent{cfg: cfg, app: app, state: app.state, tracker: tracker}
}

// Name 实现 Component。
func (a *AdminComponent) Name() string { return "admin" }

// Start 实现 Component。它同步绑定监听器。
func (a *AdminComponent) Start(_ context.Context) error {
	if !a.cfg.enabled {
		return nil
	}
	// port 为 0 让 OS 分配空闲端口；常用于测试。负端口作为配置错误拒绝。
	if a.cfg.port < 0 {
		return fmt.Errorf("admin: invalid port %d", a.cfg.port)
	}
	addr := net.JoinHostPort(a.cfg.ip, strconv.Itoa(a.cfg.port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("admin: listen %s: %w", addr, err)
	}
	a.ln = ln
	a.addr = ln.Addr().String()
	a.srv = &http.Server{
		Addr:    addr,
		Handler: a.buildMux(),
	}
	go func() {
		logger.Infof("%s admin server listening on %s", a.cfg.serviceName, a.addr)
		if err := a.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("%s admin server stopped with error | %v", a.cfg.serviceName, err)
		}
	}()
	return nil
}

// Drain 实现 Drainer。它给在途 admin 请求一个宽限窗口完成，避免在 scraper 中途抓
// 取时把端点从其下抽走。
func (a *AdminComponent) Drain(ctx context.Context) error {
	if a.srv == nil {
		return nil
	}
	drainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return a.srv.Shutdown(drainCtx)
}

// Stop 实现 Component。Drain 可能已调用过 Shutdown；Close 幂等，并强制关闭任何剩
// 余连接。
func (a *AdminComponent) Stop(_ context.Context) error {
	if a.srv == nil {
		return nil
	}
	err := a.srv.Close()
	a.srv = nil
	return err
}

func (a *AdminComponent) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/readyz", a.handleReadyz)
	mux.HandleFunc("/statez", a.handleStatez)
	mux.HandleFunc("/info", a.handleInfo)
	mux.HandleFunc("/components", a.handleComponents)
	mux.Handle("/metrics", promhttp.Handler())
	if a.cfg.pprof {
		registerPprofHandlers(mux)
	}
	return mux
}

func (a *AdminComponent) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%s admin endpoints: /healthz /readyz /statez /info /components /metrics\n", a.cfg.serviceName)
}

func (a *AdminComponent) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	st := a.app.State()
	if code := healthCode(st); code != 200 {
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (a *AdminComponent) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	st := a.app.State()
	if code := readyCode(st); code != 200 {
		http.Error(w, "not ready", code)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ready\n"))
}

func (a *AdminComponent) handleStatez(w http.ResponseWriter, _ *http.Request) {
	snap := a.state.snapshot(a.cfg.serviceName, a.app.startedAtLocked())
	writeJSON(w, http.StatusOK, snap)
}

func (a *AdminComponent) handleInfo(w http.ResponseWriter, _ *http.Request) {
	st, since := a.state.Current()
	writeJSON(w, http.StatusOK, map[string]any{
		"service":     a.cfg.serviceName,
		"state":       st,
		"state_since": formatTime(since),
		"ready":       readyCode(st) == 200,
		"alive":       st != StateStopped && st != StateFailed,
	})
}

func (a *AdminComponent) handleComponents(w http.ResponseWriter, _ *http.Request) {
	var report []ComponentReport
	if a.tracker != nil {
		report = a.tracker.Report()
	}
	writeJSON(w, http.StatusOK, struct {
		Service    string             `json:"service"`
		Components []ComponentReport `json:"components"`
	}{
		Service:    a.cfg.serviceName,
		Components: report,
	})
}

func registerPprofHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Errorf("write admin json response failed | %v", err)
	}
}
