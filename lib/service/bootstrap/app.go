package bootstrap

import (
	"context"
	"fmt"
	"runtime"
	runtimeDebug "runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
)

const defaultShutdownTimeout = 5 * time.Second

type LoggerConfig struct {
	Dir   string
	Level string
	Name  string
}

type Options struct {
	ServiceName     string
	ShutdownTimeout time.Duration

	LoadConfig        func() error
	BuildInfo         func() BuildInfo
	LoggerConfig      func() LoggerConfig
	AdminConfig       func() AdminConfig
	ComponentStatuses func() []ComponentStatus
	InitDeps          func() error
	RegisterHandlers  func() error
	StartRuntime      func() error
	OnProc            func() bool
	OnTick            func(lastMs, nowMs int64)
	// OnShutdown runs before admin shutdown / OnExit and should perform
	// graceful, timeout-bound runtime stop work.
	OnShutdown func(ctx context.Context) error
	// OnExit runs after graceful shutdown attempts and is meant for final
	// best-effort cleanup/logging.
	OnExit func()
}

type ServiceApp struct {
	options           Options
	admin             *AdminServer
	loggerInitialized bool
	alive             atomic.Bool
	ready             atomic.Bool
	stateMu           sync.RWMutex
	startedAt         time.Time
	phase             string
	phaseSince        time.Time
	lastReloadAt      time.Time
	lastError         string
	components        map[string]ComponentStatus
}

const (
	phaseCreated             = "created"
	phaseLoadingConfig       = "loading_config"
	phaseInitializingLogger  = "initializing_logger"
	phaseStartingAdmin       = "starting_admin"
	phaseInitializingDeps    = "initializing_dependencies"
	phaseRegisteringHandlers = "registering_handlers"
	phaseStartingRuntime     = "starting_runtime"
	phaseReady               = "ready"
	phaseReloading           = "reloading"
	phaseShuttingDown        = "shutting_down"
	phaseStopped             = "stopped"
	phaseInitFailed          = "init_failed"
	componentStatePending    = "pending"
	componentStateRunning    = "running"
	componentStateReady      = "ready"
	componentStateFailed     = "failed"
	componentStateSkipped    = "skipped"
	componentStateStopped    = "stopped"
	componentConfig          = "config"
	componentLogger          = "logger"
	componentAdminServer     = "admin_server"
	componentDependencies    = "dependencies"
	componentHandlers        = "handlers"
	componentRuntime         = "runtime"
)

func NewServiceApp(opts Options) *ServiceApp {
	a := &ServiceApp{options: opts}
	a.resetLifecycleState(time.Time{})
	return a
}

func (a *ServiceApp) OnInit() error {
	runtime.GOMAXPROCS(runtime.NumCPU() + 1)
	a.resetLifecycleState(time.Now())
	a.alive.Store(true)
	a.ready.Store(false)

	a.setPhase(phaseLoadingConfig)
	a.setComponentStatus(componentConfig, componentStateRunning, false, "loading configuration")
	if err := a.loadConfig(); err != nil {
		a.markComponentFailed(componentConfig, err)
		a.failInit(err)
		return err
	}
	a.setComponentStatus(componentConfig, componentStateReady, true, "configuration loaded")

	a.setPhase(phaseInitializingLogger)
	if a.options.LoggerConfig == nil {
		a.setComponentStatus(componentLogger, componentStateSkipped, true, "logger config hook not configured")
	} else {
		a.setComponentStatus(componentLogger, componentStateRunning, false, "initializing logger")
		if err := a.initLogger(); err != nil {
			a.markComponentFailed(componentLogger, err)
			a.failInit(err)
			return err
		}
		a.setComponentStatus(componentLogger, componentStateReady, true, "logger initialized")
	}

	a.setPhase(phaseStartingAdmin)
	if err := a.startAdmin(); err != nil {
		a.failInit(err)
		return err
	}
	if err := a.runStep("init deps", componentDependencies, phaseInitializingDeps, a.options.InitDeps); err != nil {
		a.failInit(err)
		a.cleanupOnInitError()
		return err
	}
	if err := a.runStep("register handlers", componentHandlers, phaseRegisteringHandlers, a.options.RegisterHandlers); err != nil {
		a.failInit(err)
		a.cleanupOnInitError()
		return err
	}
	if err := a.runStep("start runtime", componentRuntime, phaseStartingRuntime, a.options.StartRuntime); err != nil {
		a.failInit(err)
		a.cleanupOnInitError()
		return err
	}

	a.ready.Store(true)
	a.clearLastError()
	a.setPhase(phaseReady)
	logger.Infof("%s init success", a.serviceName())
	return nil
}

func (a *ServiceApp) OnReload() error {
	previousPhase := a.currentPhase()
	if previousPhase == "" {
		previousPhase = phaseReady
	}
	a.setPhase(phaseReloading)
	a.setComponentStatus(componentConfig, componentStateRunning, false, "reloading configuration")
	if err := a.loadConfig(); err != nil {
		a.markComponentFailed(componentConfig, err)
		a.setLastError(err)
		a.setPhase(previousPhase)
		return err
	}
	a.setComponentStatus(componentConfig, componentStateReady, true, "configuration reloaded")

	if a.options.LoggerConfig == nil {
		a.setComponentStatus(componentLogger, componentStateSkipped, true, "logger config hook not configured")
	} else {
		a.setComponentStatus(componentLogger, componentStateRunning, false, "reloading logger configuration")
		if err := a.initLogger(); err != nil {
			a.markComponentFailed(componentLogger, err)
			a.setLastError(err)
			a.setPhase(previousPhase)
			return err
		}
		a.setComponentStatus(componentLogger, componentStateReady, true, "logger reloaded")
	}

	a.setLastReloadAt(time.Now())
	a.clearLastError()
	a.setPhase(phaseReady)
	logger.Infof("%s reload success", a.serviceName())
	return nil
}

func (a *ServiceApp) OnProc() bool {
	if a.options.OnProc == nil {
		return true
	}
	return a.options.OnProc()
}

func (a *ServiceApp) OnTick(lastMs, nowMs int64) {
	if a.options.OnTick != nil {
		a.options.OnTick(lastMs, nowMs)
	}
}

func (a *ServiceApp) OnExit() {
	a.ready.Store(false)
	a.alive.Store(false)
	a.setPhase(phaseShuttingDown)

	if err := a.shutdownRuntime(); err != nil {
		a.markComponentFailed(componentRuntime, err)
		a.setLastError(err)
		logger.Errorf("%s runtime shutdown error | %v", a.serviceName(), err)
	} else {
		a.setComponentStatus(componentRuntime, componentStateStopped, false, "runtime shutdown complete")
	}
	if err := a.shutdownAdmin(); err != nil {
		a.setLastError(err)
		logger.Errorf("%s admin shutdown error | %v", a.serviceName(), err)
	}
	if a.options.OnExit != nil {
		a.options.OnExit()
	}
	a.setPhase(phaseStopped)
	logger.Infof("%s shutdown complete", a.serviceName())
	logger.Flush()
}

func (a *ServiceApp) shutdownRuntime() error {
	if a.options.OnShutdown == nil {
		return nil
	}
	timeout := a.options.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return a.options.OnShutdown(ctx)
}

func (a *ServiceApp) loadConfig() error {
	if a.options.LoadConfig != nil {
		if err := a.options.LoadConfig(); err != nil {
			logger.Errorf("%s load config failed | %v", a.serviceName(), err)
			return err
		}
	}
	return nil
}

func (a *ServiceApp) initLogger() error {
	if a.options.LoggerConfig == nil {
		return nil
	}
	cfg := a.options.LoggerConfig()
	if _, err := logger.InitLogger(cfg.Dir, cfg.Level, cfg.Name); err != nil {
		return err
	}
	a.loggerInitialized = true
	return nil
}

func (a *ServiceApp) startAdmin() error {
	if a.options.AdminConfig == nil {
		a.setComponentStatus(componentAdminServer, componentStateSkipped, true, "admin config hook not configured")
		return nil
	}
	cfg := a.options.AdminConfig()
	if !cfg.Enabled {
		a.setComponentStatus(componentAdminServer, componentStateSkipped, true, "admin server disabled")
		return nil
	}
	a.setComponentStatus(componentAdminServer, componentStateRunning, false, "starting admin server")
	admin, err := StartAdminServer(cfg, AdminState{
		IsAlive:    func() bool { return a.alive.Load() },
		IsReady:    func() bool { return a.ready.Load() },
		Info:       a.infoSnapshot,
		Components: a.componentsSnapshot,
	})
	if err != nil {
		a.markComponentFailed(componentAdminServer, err)
		return err
	}
	a.admin = admin
	message := "admin server started"
	if admin != nil {
		message = fmt.Sprintf("listening on %s", admin.addr)
	}
	a.setComponentStatus(componentAdminServer, componentStateReady, true, message)
	return nil
}

func (a *ServiceApp) shutdownAdmin() error {
	if a.admin == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	err := a.admin.Shutdown(ctx)
	a.admin = nil
	if err == nil {
		a.setComponentStatus(componentAdminServer, componentStateStopped, false, "admin server shutdown complete")
	}
	return err
}

func (a *ServiceApp) cleanupOnInitError() {
	a.ready.Store(false)
	a.alive.Store(false)
	if err := a.shutdownAdmin(); err != nil {
		logger.Errorf("%s cleanup admin shutdown error | %v", a.serviceName(), err)
	}
}

func (a *ServiceApp) runStep(step, component, phase string, fn func() error) error {
	a.setPhase(phase)
	if fn == nil {
		a.setComponentStatus(component, componentStateSkipped, true, fmt.Sprintf("%s hook not configured", step))
		return nil
	}
	a.setComponentStatus(component, componentStateRunning, false, step)
	if err := fn(); err != nil {
		a.markComponentFailed(component, err)
		logger.Errorf("%s %s failed | %v", a.serviceName(), step, err)
		return err
	}
	a.setComponentStatus(component, componentStateReady, true, fmt.Sprintf("%s complete", step))
	return nil
}

func (a *ServiceApp) serviceName() string {
	if a.options.ServiceName == "" {
		return "service"
	}
	return a.options.ServiceName
}

func (a *ServiceApp) resetLifecycleState(startedAt time.Time) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	phase := phaseCreated
	phaseSince := time.Time{}
	if !startedAt.IsZero() {
		phase = phaseLoadingConfig
		phaseSince = startedAt
	}

	a.startedAt = startedAt
	a.phase = phase
	a.phaseSince = phaseSince
	a.lastReloadAt = time.Time{}
	a.lastError = ""
	a.components = map[string]ComponentStatus{
		componentConfig: {
			Name:    componentConfig,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
		componentLogger: {
			Name:    componentLogger,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
		componentAdminServer: {
			Name:    componentAdminServer,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
		componentDependencies: {
			Name:    componentDependencies,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
		componentHandlers: {
			Name:    componentHandlers,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
		componentRuntime: {
			Name:    componentRuntime,
			State:   componentStatePending,
			Message: "awaiting initialization",
		},
	}
}

func (a *ServiceApp) setPhase(phase string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.phase = phase
	a.phaseSince = time.Now().UTC()
}

func (a *ServiceApp) currentPhase() string {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.phase
}

func (a *ServiceApp) setLastError(err error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if err == nil {
		a.lastError = ""
		return
	}
	a.lastError = err.Error()
}

func (a *ServiceApp) clearLastError() {
	a.setLastError(nil)
}

func (a *ServiceApp) setLastReloadAt(ts time.Time) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.lastReloadAt = ts.UTC()
}

func (a *ServiceApp) failInit(err error) {
	a.ready.Store(false)
	a.alive.Store(false)
	a.setLastError(err)
	a.setPhase(phaseInitFailed)
}

func (a *ServiceApp) setComponentStatus(name, state string, ready bool, message string) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	status := a.components[name]
	status.Name = name
	status.State = state
	status.Ready = ready
	status.Message = message
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	a.components[name] = status
}

func (a *ServiceApp) markComponentFailed(name string, err error) {
	message := "component failed"
	if err != nil {
		message = err.Error()
	}
	a.setComponentStatus(name, componentStateFailed, false, message)
}

func (a *ServiceApp) infoSnapshot() InfoSnapshot {
	a.stateMu.RLock()
	startedAt := a.startedAt
	phase := a.phase
	phaseSince := a.phaseSince
	lastReloadAt := a.lastReloadAt
	lastError := a.lastError
	a.stateMu.RUnlock()

	build := a.defaultBuildInfo()
	if a.options.BuildInfo != nil {
		build = mergeBuildInfo(build, a.options.BuildInfo())
	}

	info := InfoSnapshot{
		Service:      a.serviceName(),
		Alive:        a.alive.Load(),
		Ready:        a.ready.Load(),
		Phase:        phase,
		StartedAt:    formatAdminTime(startedAt),
		PhaseSince:   formatAdminTime(phaseSince),
		LastReloadAt: formatAdminTime(lastReloadAt),
		LastError:    lastError,
		Build:        build,
	}
	if !startedAt.IsZero() {
		info.Uptime = time.Since(startedAt).Round(time.Second).String()
	}
	return info
}

func (a *ServiceApp) componentsSnapshot() []ComponentStatus {
	a.stateMu.RLock()
	components := make([]ComponentStatus, 0, len(a.components))
	for _, status := range a.components {
		components = append(components, status)
	}
	a.stateMu.RUnlock()

	if a.options.ComponentStatuses != nil {
		components = mergeComponentStatuses(components, a.options.ComponentStatuses())
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})
	return components
}

func (a *ServiceApp) defaultBuildInfo() BuildInfo {
	info := BuildInfo{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
	buildInfo, ok := runtimeDebug.ReadBuildInfo()
	if !ok {
		return info
	}
	if buildInfo.Path != "" {
		info.Path = buildInfo.Path
	}
	if buildInfo.Main.Version != "" {
		info.Version = buildInfo.Main.Version
	}
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			info.VCSRevision = setting.Value
		case "vcs.time":
			info.VCSTime = setting.Value
		case "vcs.modified":
			info.VCSDirty = setting.Value
		}
	}
	return info
}

func mergeBuildInfo(base, override BuildInfo) BuildInfo {
	if override.Path != "" {
		base.Path = override.Path
	}
	if override.Version != "" {
		base.Version = override.Version
	}
	if override.GoVersion != "" {
		base.GoVersion = override.GoVersion
	}
	if override.GOOS != "" {
		base.GOOS = override.GOOS
	}
	if override.GOARCH != "" {
		base.GOARCH = override.GOARCH
	}
	if override.VCSRevision != "" {
		base.VCSRevision = override.VCSRevision
	}
	if override.VCSTime != "" {
		base.VCSTime = override.VCSTime
	}
	if override.VCSDirty != "" {
		base.VCSDirty = override.VCSDirty
	}
	return base
}

func mergeComponentStatuses(base, overrides []ComponentStatus) []ComponentStatus {
	if len(overrides) == 0 {
		return base
	}
	merged := make(map[string]ComponentStatus, len(base)+len(overrides))
	for _, status := range base {
		merged[status.Name] = status
	}
	for _, status := range overrides {
		if status.Name == "" {
			continue
		}
		merged[status.Name] = status
	}
	result := make([]ComponentStatus, 0, len(merged))
	for _, status := range merged {
		result = append(result, status)
	}
	return result
}

func formatAdminTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}
