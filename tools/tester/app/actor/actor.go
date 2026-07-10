// Package actor 回归测试模拟玩家：组合 session 会话层与业务测试组件。
package actor

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
)

type State int32

const (
	StateInit State = iota
	StateConnecting
	StateConnected
	StateAccountLogin
	StateRoleLogin
	StateTesting
	StateDone
	StateError
)

type ClientActor struct {
	id  int
	cfg *testcfg.Config

	moduleNames []string

	sess  *session.Session
	state atomic.Int32

	components []component.TesterComponent

	doneCh    chan struct{}
	lastError error
}

// NewClientActor 创建回归测试模拟玩家；collector 可为 nil（不统计）。
func NewClientActor(id int, cfg *testcfg.Config, moduleNames []string, collector *stats.Collector, serverID int32) *ClientActor {
	a := &ClientActor{
		id:          id,
		cfg:         cfg,
		moduleNames: moduleNames,
		doneCh:      make(chan struct{}),
	}
	a.state.Store(int32(StateInit))

	a.sess = session.New(session.Options{
		ID:         id,
		Transport:  cfg.Server.Transport,
		Host:       cfg.Server.Host,
		TcpPort:    cfg.Server.TcpPort,
		WsPort:     cfg.Server.WsPort,
		WsPath:     cfg.Server.WsPath,
		Channel:    cfg.Player.Channel,
		AccountID:  fmt.Sprintf("%s_%d", cfg.Player.AccountPrefix, id),
		DeviceID:   fmt.Sprintf("%s_%d", cfg.Player.DevicePrefix, id),
		UserID:     cfg.Player.StartUID + int64(id),
		Token:      cfg.Player.Token,
		Collector:  collector,
	})

	return a
}

func (a *ClientActor) ID() int {
	return a.id
}

func (a *ClientActor) State() State {
	return State(a.state.Load())
}

func (a *ClientActor) AccountID() string {
	return a.sess.AccountID()
}

func (a *ClientActor) UserID() int64 {
	return a.sess.UserID()
}

func (a *ClientActor) LastError() error {
	return a.lastError
}

func (a *ClientActor) Done() <-chan struct{} {
	return a.doneCh
}

func (a *ClientActor) Session() *session.Session {
	return a.sess
}

func (a *ClientActor) Run(ctx context.Context) error {
	if err := a.initComponents(); err != nil {
		return err
	}

	a.state.Store(int32(StateConnecting))
	if err := a.sess.Connect(ctx); err != nil {
		a.state.Store(int32(StateError))
		return fmt.Errorf("actor %d: %w", a.id, err)
	}
	a.state.Store(int32(StateConnected))
	for _, comp := range a.components {
		comp.OnConnected()
	}

	a.state.Store(int32(StateAccountLogin))
	if err := a.sess.Login(ctx); err != nil {
		a.state.Store(int32(StateError))
		return fmt.Errorf("actor %d: %w", a.id, err)
	}
	for _, comp := range a.components {
		comp.OnAccountLogin(a.sess.AccountID())
		comp.OnRoleLogin(a.sess.UserID())
	}

	if err := a.runTests(ctx); err != nil {
		a.state.Store(int32(StateError))
		a.lastError = err
		return err
	}

	a.state.Store(int32(StateDone))
	close(a.doneCh)
	return nil
}

func (a *ClientActor) Close() {
	a.sess.Close()
}

func (a *ClientActor) initComponents() error {
	for _, name := range a.moduleNames {
		comp, err := component.Create(name)
		if err != nil {
			return fmt.Errorf("actor %d: create component %q: %w", a.id, name, err)
		}

		compCtx := &component.ComponentContext{
			ActorID:   a.id,
			AccountID: a.sess.AccountID(),
			UserID:    a.sess.UserID(),
			Sender:    a.sess,
			Requester: a.sess,
			Cfg:       a.cfg,
		}

		if err := comp.OnInit(compCtx); err != nil {
			return fmt.Errorf("actor %d: init component %q: %w", a.id, name, err)
		}

		a.sess.OnMessage(comp.OnMessage)
		a.components = append(a.components, comp)
	}

	return nil
}

func (a *ClientActor) runTests(ctx context.Context) error {
	a.state.Store(int32(StateTesting))

	for _, comp := range a.components {
		a.sess.SetModule(comp.Name())
		err := comp.RunTests(ctx)
		a.sess.SetModule("core")
		if err != nil {
			return fmt.Errorf("actor %d: component %q test failed: %w", a.id, comp.Name(), err)
		}
	}

	return nil
}
