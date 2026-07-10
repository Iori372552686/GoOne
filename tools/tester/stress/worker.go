package stress

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
)

// worker 单玩家单协程：连接 → 登录 → 业务循环（随机/顺序）→ 优雅退出。
// 连接断开时自动重连重登；单轮业务失败计入统计但不中止。
type worker struct {
	slot int // 玩家槽位，决定 UID/账号后缀
	ctl  *Controller

	cancel context.CancelFunc
	doneCh chan struct{}
}

func newWorker(slot int, ctl *Controller) *worker {
	return &worker{
		slot:   slot,
		ctl:    ctl,
		doneCh: make(chan struct{}),
	}
}

// run 由 Controller 在独立协程中调用。
func (w *worker) run(parent context.Context) {
	defer close(w.doneCh)

	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	defer cancel()

	const reconnectWait = 3 * time.Second

	for ctx.Err() == nil {
		err := w.runSession(ctx)
		if ctx.Err() != nil || err == nil {
			return // 正常结束（停止信号或单轮模式完成）
		}
		log.Printf("[Stress][P%d] session ended: %v, reconnecting in %v", w.slot, err, reconnectWait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectWait):
		}
	}
}

// runSession 一次完整会话生命周期；返回 nil 表示工作完成（无需重连）。
func (w *worker) runSession(ctx context.Context) error {
	cfg := w.ctl.cfg

	sess := session.New(session.Options{
		ID:         w.slot,
		Transport:  cfg.Server.Transport,
		Host:       cfg.Server.Host,
		TcpPort:    cfg.Server.TcpPort,
		WsPort:     cfg.Server.WsPort,
		WsPath:     cfg.Server.WsPath,
		Channel:    cfg.Player.Channel,
		AccountID:  fmt.Sprintf("%s_%d", cfg.Player.AccountPrefix, w.slot),
		DeviceID:   fmt.Sprintf("%s_%d", cfg.Player.DevicePrefix, w.slot),
		UserID:     cfg.Player.StartUID + int64(w.slot),
		Token:      cfg.Player.Token,
		Collector:  w.ctl.collector,
	})
	defer sess.Close()

	// 组件实例每次会话重建（状态与连接绑定）
	runners, err := w.buildComponents(sess)
	if err != nil {
		return err
	}

	if err := sess.Connect(ctx); err != nil {
		return err
	}
	for _, r := range runners {
		r.comp.OnConnected()
	}

	if err := sess.Login(ctx); err != nil {
		return err
	}
	for _, r := range runners {
		r.comp.OnAccountLogin(sess.AccountID())
		r.comp.OnRoleLogin(sess.UserID())
	}

	w.ctl.collector.AddOnline(1)
	defer w.ctl.collector.AddOnline(-1)

	return w.businessLoop(ctx, sess, runners)
}

type stressRunner struct {
	name   string
	weight int
	comp   component.TesterComponent
	runner component.StressRunner
}

func (w *worker) buildComponents(sess *session.Session) ([]*stressRunner, error) {
	cfg := w.ctl.cfg
	runners := make([]*stressRunner, 0, len(w.ctl.modules))

	for _, name := range w.ctl.modules {
		comp, err := component.Create(name)
		if err != nil {
			return nil, fmt.Errorf("P%d: create component %q: %w", w.slot, name, err)
		}
		sr, ok := comp.(component.StressRunner)
		if !ok {
			// Controller 启动时已过滤，理论到不了这里
			continue
		}

		compCtx := &component.ComponentContext{
			ActorID:   w.slot,
			AccountID: sess.AccountID(),
			UserID:    sess.UserID(),
			Sender:    sess,
			Requester: sess,
			Cfg:       cfg,
		}
		if err := comp.OnInit(compCtx); err != nil {
			return nil, fmt.Errorf("P%d: init component %q: %w", w.slot, name, err)
		}
		sess.OnMessage(comp.OnMessage)

		runners = append(runners, &stressRunner{
			name:   name,
			weight: cfg.ModuleSetting(name).Weight,
			comp:   comp,
			runner: sr,
		})
	}

	if len(runners) == 0 {
		return nil, fmt.Errorf("P%d: no stress-capable module enabled", w.slot)
	}
	return runners, nil
}

// businessLoop 业务循环；返回 nil 表示自然结束，返回 error 触发重连。
func (w *worker) businessLoop(ctx context.Context, sess *session.Session, runners []*stressRunner) error {
	cfg := w.ctl.cfg
	thinkTime := time.Duration(cfg.Stress.ThinkTimeMs) * time.Millisecond
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(w.slot)))

	totalWeight := 0
	for _, r := range runners {
		totalWeight += r.weight
	}

	seqIdx := 0
	singleRound := !cfg.Stress.Loop

	for round := 0; ; round++ {
		if ctx.Err() != nil {
			return nil
		}
		if !sess.Connected() {
			return fmt.Errorf("connection lost")
		}

		// 暂停：循环间隙轮询
		for w.ctl.paused.Load() {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(200 * time.Millisecond):
			}
		}

		var r *stressRunner
		if cfg.Stress.Flow == testcfg.FlowSequential {
			r = runners[seqIdx%len(runners)]
			seqIdx++
		} else {
			pick := rng.Intn(totalWeight)
			for _, cand := range runners {
				pick -= cand.weight
				if pick < 0 {
					r = cand
					break
				}
			}
		}

		sess.SetModule(r.name)
		start := time.Now()
		err := r.runner.RunStress(ctx)
		elapsed := time.Since(start)
		sess.SetModule("core")

		if ctx.Err() != nil {
			return nil
		}
		w.ctl.collector.RecordLoop(r.name, err == nil, elapsed)
		if err != nil {
			w.ctl.collector.RecordError(r.name, "", fmt.Sprintf("P%d round %d: %v", w.slot, round, err))
			if !sess.Connected() {
				return fmt.Errorf("connection lost during %s: %w", r.name, err)
			}
		}

		// 单轮模式：每个模块跑一遍后结束
		if singleRound && (seqIdx >= len(runners) || (cfg.Stress.Flow == testcfg.FlowRandom && round+1 >= len(runners))) {
			return nil
		}

		if thinkTime > 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(thinkTime):
			}
		}
	}
}
