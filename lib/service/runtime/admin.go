package runtime

import (
	"context"
	"crypto/tls"
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
	// readyCheck 是额外的就绪探针（如 router.ReadyCheck）；readyz 在 lifecycle Ready
	// 时叠加调用它。可不设。
	readyCheck func() error
	// configSource 在 Start 时被调用一次，返回最新的 AdminConfig；用于在 LoadConfig
	// 之后才确定端口/IP 的场景。nil 表示使用构造期已设置的静态值。
	configSource func() AdminConfig
}

// AdminConfig 是 admin server 的可外部配置面。生产服务通过 WithAdminConfig 注入一
// 个返回 AdminConfig 的闭包；该闭包在 AdminComponent.Start 时被调用一次，使端口/IP
// 可以读取 LoadConfig 之后才生效的配置值。
type AdminConfig struct {
	Enabled bool
	IP      string
	Port    int
	Pprof   bool
}

// WithAdminConfig 安装一个在 AdminComponent.Start 时调用一次的配置源。source 返回的
// AdminConfig 覆盖构造期的 WithAdminListen/WithAdminPprof 值；这是生产服务的推荐接
// 线方式，因为 admin 端口/IP 通常依赖 LoadConfig 之后才确定的配置。
//
// source 为 nil 时按已设置的静态选项工作。WithAdminReadyCheck 与 WithAdminPprof 不
// 隐式启用 admin；只有 Enabled（直接或经 source）为 true 时才绑定监听器。
func WithAdminConfig(source func() AdminConfig) AdminOption {
	return func(c *adminConfig) {
		c.configSource = source
	}
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

// WithAdminReadyCheck 注入一个额外的就绪探针（如 router.ReadyCheck）。readyz 在
// lifecycle 状态为 Ready/Allocated 时，额外调用它；返回非 nil error 则 readyz 返回
// 503（用于 bus 断连等运行期故障自动摘流）。可不设。
func WithAdminReadyCheck(fn func() error) AdminOption {
	return func(c *adminConfig) {
		c.readyCheck = fn
		c.enabled = true
	}
}

// AdminComponent 把一个 HTTP admin server 包装为 Component。它暴露
// /healthz、/readyz、/statez、/info、/components 与 /metrics，可选 pprof。它实现
// Drainer（HTTP Shutdown），使在途 admin 请求在排空期获得宽限窗口。
//
// Start 同步绑定监听器，使端口冲突在启动期而非稍后暴露。Stop 带超时关闭 server。
//
// P0-03：tracker 直接取自 app.tracker（runtime.New 默认创建），不再由调用方外部传
// 入，使 /components 在任何接线上都能列出 pending/running 组件。
type AdminComponent struct {
	cfg     adminConfig
	app     *App
	state   *StateStore
	tracker *ComponentTracker
	srv     *http.Server
	ln      net.Listener
	addr    string
	// runtimeErrCh 汇聚 admin server 的运行期错误（如 Serve 异常退出），实现
	// RuntimeErrorSource 供 App 监督。容量 1，使 Serve goroutine 在 App 订阅前失败也
	// 不丢错误。
	runtimeErrCh chan error
}

// NewAdminComponent 构建一个绑定到给定 App 的 state store 与 component tracker
// 的 admin Component。必须在 Run 之前构造。
//
// P0-03：tracker 参数已移除；admin 直接使用 app.tracker（runtime.New 默认创建）。
// 旧的 NewAdminComponent(app, tracker, ...) 调用方改为 NewAdminComponent(app, ...)。
func NewAdminComponent(app *App, opts ...AdminOption) *AdminComponent {
	cfg := adminConfig{serviceName: app.Name()}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.ip == "" {
		cfg.ip = "127.0.0.1"
	}
	return &AdminComponent{
		cfg:          cfg,
		app:          app,
		state:        app.state,
		tracker:      app.tracker,
		runtimeErrCh: make(chan error, 1),
	}
}

// Name 实现 Component。
func (a *AdminComponent) Name() string { return "admin" }

// RuntimeErrors 实现 RuntimeErrorSource。admin server 的 Serve goroutine 在异常退出
// （非 http.ErrServerClosed）时把错误送入此 channel，使 App 能监督监听器死亡并触发标
// 准关停。预期的 http.ErrServerClosed 不上报。
func (a *AdminComponent) RuntimeErrors() <-chan error {
	return a.runtimeErrCh
}

// Start 实现 Component。它同步绑定监听器。
//
// P0-03：若设置了 WithAdminConfig 的 source，在 Start 时调用一次以读取 LoadConfig
// 之后才生效的端口/IP/Enabled/Pprof，覆盖构造期的静态值。这修正了历史上"构造期冻结
// cfg.ip/cfg.port、无法反映 LoadConfig 后配置"的问题。
func (a *AdminComponent) Start(_ context.Context) error {
	// 先应用 configSource（若设置），使端口/IP 来自 LoadConfig 后的值。
	if a.cfg.configSource != nil {
		applied := a.cfg.configSource()
		a.cfg.enabled = applied.Enabled
		a.cfg.pprof = applied.Pprof
		if applied.IP != "" {
			a.cfg.ip = applied.IP
		}
		if applied.Port != 0 {
			a.cfg.port = applied.Port
		}
	}
	if !a.cfg.enabled {
		return nil
	}
	// 空 IP 默认回环地址，绝不绑定所有接口（admin 含 pprof，不能对外暴露）。
	if a.cfg.ip == "" {
		a.cfg.ip = "127.0.0.1"
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
		// admin 端点为明文 HTTP；显式禁用 HTTP/2，避免 net/http 在某些 Go 版本上
		// setupHTTP2_Serve 对未配置 TLS 的 server 触发 nil 解引用。
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	go func() {
		logger.Infof("%s admin server listening on %s", a.cfg.serviceName, a.addr)
		// 防御：a.srv 在 Start 内赋值，理论上 goroutine 启动时已就绪；但若 Start 被
		// 并发误调或被外部重置，避免 nil 解引用 panic 杀死测试进程。
		if a.srv == nil {
			logger.Errorf("%s admin server: srv is nil, cannot Serve", a.cfg.serviceName)
			return
		}
		if err := a.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Errorf("%s admin server stopped with error | %v", a.cfg.serviceName, err)
			// 上报非预期退出，触发 App 监督关停。channel 容量 1，不阻塞。
			select {
			case a.runtimeErrCh <- err:
			default:
			}
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
	// lifecycle Ready 时叠加运行期就绪探针（如 router.ReadyCheck）：bus 断连等故障
	// 使 readyz 返回 503 自动摘流。
	if a.cfg.readyCheck != nil {
		if err := a.cfg.readyCheck(); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ready\n"))
}

func (a *AdminComponent) handleStatez(w http.ResponseWriter, _ *http.Request) {
	snap := a.state.snapshot(a.cfg.serviceName)
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
		Service    string            `json:"service"`
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
