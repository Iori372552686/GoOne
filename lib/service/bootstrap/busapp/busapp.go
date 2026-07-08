// Package busapp assembles a standard GoOne bus-attached service
// (connsvr/mainsvr/infosvr/mysqlsvr/roomcentersvr) on top of bootstrap.
//
// The five bus services shared ~70% identical wiring code (tracing init,
// transaction manager startup, router startup, readiness probe and the
// graceful-shutdown sequence). busapp centralizes that wiring; a service's
// app.go only declares its config accessors and its business-specific hooks.
package busapp

import (
	"context"
	"errors"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/misc"
)

// Common carries the config sections shared by every bus service.
// It is produced by the service's Common() accessor after LoadConfig.
type Common struct {
	LogDir   string
	LogLevel string

	SelfBusId    string
	BusMQAddr    string
	RegisterAddr string

	AdminEnabled bool
	AdminIP      string
	AdminPort    int
	Pprof        bool

	Tracing ssrpc.TracingConfig
}

// Options declares the service-specific parts of a bus service.
type Options struct {
	ServiceName string
	ServerType  int

	// LoadConfig loads and validates the service config file. Required.
	LoadConfig func() error
	// Common returns the shared config sections. Required; called after
	// LoadConfig has succeeded.
	Common func() Common

	// TransMgr is the service's transaction manager singleton. Required.
	TransMgr transaction.ITransactionMgr
	// TransConfig, when set, overrides the default single-shard,
	// non-serialized transaction setup.
	TransConfig func() transaction.TransactionMgrConfig
	// OnRecvSSPacket, when set, replaces the default behavior of forwarding
	// every bus packet to TransMgr (connsvr needs this to short-circuit
	// client-bound packets).
	OnRecvSSPacket func(*sharedstruct.SSPacket)

	// InitDeps initializes service-specific dependencies (redis, gamedata,
	// idgen, ...). Optional; runs after tracing is initialized.
	InitDeps func() error
	// RegisterHandlers registers the service's ssrpc dispatchers. Required.
	RegisterHandlers func() error
	// StartExtra runs after TransMgr and router are up (gateway listeners,
	// self-message senders, background loops, ...). Optional.
	StartExtra func() error

	ComponentStatuses func() []bootstrap.ComponentStatus
	OnProc            func() bool
	OnTick            func(lastMs, nowMs int64)
	// OnShutdownExtra runs during graceful shutdown, after the transaction
	// manager has been drained and before the router closes. Optional.
	OnShutdownExtra func(ctx context.Context) error
	OnExit          func()
}

// New assembles a bootstrap.ServiceApp with the standard bus-service wiring.
func New(opts Options) *bootstrap.ServiceApp {
	return bootstrap.NewServiceApp(bootstrap.Options{
		ServiceName: opts.ServiceName,
		LoadConfig:  opts.LoadConfig,
		LoggerConfig: func() bootstrap.LoggerConfig {
			c := opts.Common()
			return bootstrap.LoggerConfig{
				Dir:   c.LogDir,
				Level: c.LogLevel,
				Name:  opts.ServiceName,
			}
		},
		AdminConfig: func() bootstrap.AdminConfig {
			c := opts.Common()
			return bootstrap.NewAdminConfig(
				opts.ServiceName,
				opts.ServerType,
				c.AdminEnabled,
				c.Pprof,
				c.AdminIP,
				c.AdminPort,
			)
		},
		ComponentStatuses: opts.ComponentStatuses,
		// bus 断连时 /readyz 返回 503，摘除流量直至重连成功
		ReadyCheck: router.ReadyCheck,
		InitDeps: func() error {
			if err := ssrpc.InitTracing(opts.ServiceName, opts.Common().Tracing); err != nil {
				return err
			}
			if opts.InitDeps != nil {
				return opts.InitDeps()
			}
			return nil
		},
		RegisterHandlers: opts.RegisterHandlers,
		StartRuntime: func() error {
			if opts.TransConfig != nil {
				opts.TransMgr.InitAndRunWithConfig(opts.TransConfig())
			} else {
				opts.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)
			}

			onRecv := opts.OnRecvSSPacket
			if onRecv == nil {
				onRecv = func(packet *sharedstruct.SSPacket) {
					opts.TransMgr.ProcessSSPacket(packet)
				}
			}

			c := opts.Common()
			if err := router.InitAndRun(
				c.SelfBusId,
				onRecv,
				c.BusMQAddr,
				misc.ServerRouteRules,
				c.RegisterAddr,
			); err != nil {
				return err
			}

			if opts.StartExtra != nil {
				return opts.StartExtra()
			}
			return nil
		},
		OnProc: func() bool {
			if opts.OnProc != nil {
				return opts.OnProc()
			}
			return true
		},
		OnTick: opts.OnTick,
		OnShutdown: func(ctx context.Context) error {
			router.BeginShutdown()
			shutdownErr := opts.TransMgr.Close(ctx)

			if opts.OnShutdownExtra != nil {
				if err := opts.OnShutdownExtra(ctx); err != nil && shutdownErr == nil {
					shutdownErr = err
				}
			}

			if err := router.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
			if err := ssrpc.ShutdownTracing(ctx); err != nil {
				shutdownErr = errors.Join(shutdownErr, err)
			}
			return shutdownErr
		},
		OnExit: opts.OnExit,
	})
}
