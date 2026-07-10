// Package engine 回归测试引擎：按配置并发拉起 N 个模拟玩家，汇总 PASS/FAIL。
package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/actor"
	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
)

type TestEngine struct {
	cfg       *testcfg.Config
	collector *stats.Collector
	serverID  int32

	actors  []*actor.ClientActor
	mu      sync.Mutex
	results map[int]*ActorResult
}

type ActorResult struct {
	ActorID int
	Success bool
	Error   error
	Elapsed time.Duration
}

// NewEngine 创建回归引擎；collector 可为 nil（不统计协议延迟）。
func NewEngine(cfg *testcfg.Config, moduleNames []string, collector *stats.Collector, serverID int32) *TestEngine {
	e := &TestEngine{
		cfg:       cfg,
		collector: collector,
		serverID:  serverID,
		results:   make(map[int]*ActorResult),
	}

	for i := 0; i < cfg.Player.Players; i++ {
		a := actor.NewClientActor(i, cfg, moduleNames, collector, serverID)
		e.actors = append(e.actors, a)
	}

	return e
}

// Run 执行所有模拟玩家的回归测试，阻塞直到全部完成。
// 返回 true 表示全部通过；退出进程等收尾动作由调用方决定。
func (e *TestEngine) Run(ctx context.Context) bool {
	log.Printf("[Engine] Starting %d simulated players...", len(e.actors))

	var wg sync.WaitGroup

	// 限制并发数，避免本地 all-in-one 进程被百连接瞬间压垮导致超时
	const maxConcurrency = 10
	sem := make(chan struct{}, maxConcurrency)

	for i, a := range e.actors {
		wg.Add(1)
		go func(id int, act *actor.ClientActor) {
			defer wg.Done()

			// 错峰启动，避免瞬间百连接同时压到网关/服务注册
			time.Sleep(time.Duration(id) * 30 * time.Millisecond)

			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			err := act.Run(ctx)
			elapsed := time.Since(start)

			result := &ActorResult{
				ActorID: act.ID(),
				Success: err == nil,
				Error:   err,
				Elapsed: elapsed,
			}

			e.mu.Lock()
			e.results[act.ID()] = result
			e.mu.Unlock()

			if err != nil {
				log.Printf("[Actor %d] FAILED after %v: %v", act.ID(), elapsed, err)
			} else {
				log.Printf("[Actor %d] SUCCESS in %v", act.ID(), elapsed)
			}
		}(i, a)
	}

	wg.Wait()

	e.printSummary()

	for _, r := range e.results {
		if !r.Success {
			return false
		}
	}
	return true
}

func (e *TestEngine) Stop() {
	for _, a := range e.actors {
		a.Close()
	}
}

func (e *TestEngine) Results() map[int]*ActorResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	results := make(map[int]*ActorResult, len(e.results))
	for k, v := range e.results {
		results[k] = v
	}
	return results
}

func (e *TestEngine) printSummary() {
	e.mu.Lock()
	defer e.mu.Unlock()

	total := len(e.results)
	success := 0
	failed := 0

	for _, r := range e.results {
		if r.Success {
			success++
		} else {
			failed++
		}
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("        TEST SUMMARY                    ")
	fmt.Println("========================================")
	fmt.Printf("  Total actors:  %d\n", total)
	fmt.Printf("  Success:       %d\n", success)
	fmt.Printf("  Failed:        %d\n", failed)
	fmt.Println("----------------------------------------")

	for _, r := range e.results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		fmt.Printf("  Actor %3d: %s (%v)", r.ActorID, status, r.Elapsed)
		if r.Error != nil {
			fmt.Printf(" - %v", r.Error)
		}
		fmt.Println()
	}

	fmt.Println("========================================")
}
