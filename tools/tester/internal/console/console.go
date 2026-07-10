// Package console 压测终端可视化面板 + stdin 命令干预。
//
// 面板每秒 ANSI 清屏重绘关键指标；stdin 支持运行时命令：
// add/remove/pause/resume/stats/stop/help。
// 非 TTY 环境（CI/重定向）自动降级为周期性日志输出，命令不可用。
package console

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/Iori372552686/GoOne/tools/tester/stress"
)

// Console 终端交互器。
type Console struct {
	ctl       *stress.Controller
	collector *stats.Collector

	interactive bool
	startAt     time.Time
}

func New(ctl *stress.Controller, collector *stats.Collector) *Console {
	return &Console{
		ctl:         ctl,
		collector:   collector,
		interactive: isTerminal(),
		startAt:     time.Now(),
	}
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Run 启动面板刷新与命令读取，阻塞直到 ctx 取消。
func (c *Console) Run(ctx context.Context) {
	if c.interactive {
		go c.readCommands(ctx)
	}

	interval := time.Second
	if !c.interactive {
		interval = 10 * time.Second // 日志模式降频
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.interactive {
				c.drawPanel()
			} else {
				c.logLine()
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 命令
// ---------------------------------------------------------------------------

func (c *Console) readCommands(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		c.execCommand(line)
	}
}

func (c *Console) execCommand(line string) {
	fields := strings.Fields(strings.ToLower(line))
	cmd := fields[0]
	arg := 0
	if len(fields) > 1 {
		arg, _ = strconv.Atoi(fields[1])
	}

	switch cmd {
	case "add":
		if arg <= 0 {
			arg = 10
		}
		c.ctl.AddPlayers(arg)
	case "remove", "rm":
		if arg <= 0 {
			arg = 10
		}
		c.ctl.RemovePlayers(arg)
	case "pause":
		c.ctl.Pause()
	case "resume":
		c.ctl.Resume()
	case "stats":
		module := ""
		if len(fields) > 1 {
			module = fields[1]
		}
		c.printModuleStats(module)
	case "stop", "quit", "exit", "q":
		c.ctl.Stop("手动停止（控制台命令）")
	case "help", "h", "?":
		c.printHelp()
	default:
		fmt.Printf("未知命令 %q，输入 help 查看可用命令\n", cmd)
	}
}

func (c *Console) printHelp() {
	fmt.Println(`可用命令:
  add <n>       增加 n 个玩家（默认 10）
  remove <n>    移除 n 个玩家（默认 10）
  pause         暂停业务循环（保持连接与登录态）
  resume        恢复业务循环
  stats [模块]  打印指定模块（缺省全部）的协议明细
  stop / quit   优雅停止并生成报告
  help          显示本帮助`)
}

func (c *Console) printModuleStats(module string) {
	snap := c.collector.Snapshot()
	for _, m := range snap.Modules {
		if module != "" && m.Name != module {
			continue
		}
		fmt.Printf("--- 模块 %s: loops=%d pass=%.1f%% tps=%.2f ---\n",
			m.Name, m.Loops, m.PassRate()*100, m.TPS)
		for _, p := range m.Protos {
			fmt.Printf("  %-32s total=%-7d ok=%.1f%% avg=%-8s p95=%-8s p99=%-8s\n",
				protoName(p), p.Total, p.SuccessRate()*100,
				fmtDur(p.Avg), fmtDur(p.P95), fmtDur(p.P99))
		}
	}
}

// ---------------------------------------------------------------------------
// 面板
// ---------------------------------------------------------------------------

const (
	ansiClear = "\x1b[2J\x1b[H"
)

func (c *Console) drawPanel() {
	snap := c.collector.Snapshot()

	var b strings.Builder
	b.WriteString(ansiClear)

	state := "运行中"
	if c.ctl.Paused() {
		state = "已暂停"
	}

	b.WriteString("================================ GoOne 压力测试 ================================\n")
	fmt.Fprintf(&b, " 状态: %-8s 运行时长: %-12s 覆盖模块: %s\n",
		state, fmtDur(snap.Elapsed), strings.Join(c.ctl.Modules(), ","))
	fmt.Fprintf(&b, " 在线玩家: %-6d worker: %-6d 总请求: %-10d 总循环: %-8d 错误: %d\n",
		snap.Online, c.ctl.WorkerCount(), snap.TotalRequests, snap.TotalLoops, snap.TotalErrors)

	// 最近采样点（实时 QPS/TPS 与服务器资源）
	if n := len(snap.Samples); n > 0 {
		s := snap.Samples[n-1]
		fmt.Fprintf(&b, " 实时 QPS: %-8.1f 实时 TPS: %-8.1f 服务器 CPU: %-6s 内存: %-8s 协程: %s\n",
			s.QPS, s.TPS, fmtCPU(s.CPUCores), fmtBytes(s.HeapBytes), fmtCount(s.Goroutines))
	}

	b.WriteString("--------------------------------------------------------------------------------\n")
	b.WriteString(" 模块           循环      通过率     TPS      | Top协议延迟          avg      p99\n")

	type protoRow struct {
		name string
		avg  time.Duration
		p99  time.Duration
	}
	sortModules(snap.Modules)
	for _, m := range snap.Modules {
		if m.Name == "core" && m.Loops == 0 && len(m.Protos) == 0 {
			continue
		}
		// 该模块 p99 最高的协议
		var top *protoRow
		for _, p := range m.Protos {
			if top == nil || p.P99 > top.p99 {
				top = &protoRow{name: protoName(p), avg: p.Avg, p99: p.P99}
			}
		}
		topStr := ""
		if top != nil {
			topStr = fmt.Sprintf("%-20s %-8s %-8s", trunc(top.name, 20), fmtDur(top.avg), fmtDur(top.p99))
		}
		fmt.Fprintf(&b, " %-14s %-9d %-9s %-8.2f | %s\n",
			trunc(m.Name, 14), m.Loops, fmt.Sprintf("%.1f%%", m.PassRate()*100), m.TPS, topStr)
	}

	// 最近错误
	if len(snap.Errors) > 0 {
		b.WriteString("-------------------------------- 最近错误 --------------------------------------\n")
		start := len(snap.Errors) - 3
		if start < 0 {
			start = 0
		}
		for _, e := range snap.Errors[start:] {
			fmt.Fprintf(&b, " [%s][%s] %s\n", e.At.Format("15:04:05"), e.Module, trunc(e.Detail, 90))
		}
	}

	b.WriteString("================================================================================\n")
	b.WriteString(" 命令: add <n> | remove <n> | pause | resume | stats [模块] | stop | help\n> ")

	fmt.Print(b.String())
}

func (c *Console) logLine() {
	snap := c.collector.Snapshot()
	var rt string
	if n := len(snap.Samples); n > 0 {
		s := snap.Samples[n-1]
		rt = fmt.Sprintf(" qps=%.1f tps=%.1f cpu=%s mem=%s goroutines=%s",
			s.QPS, s.TPS, fmtCPU(s.CPUCores), fmtBytes(s.HeapBytes), fmtCount(s.Goroutines))
	}
	fmt.Printf("[Stress] elapsed=%s online=%d req=%d loops=%d errors=%d%s\n",
		fmtDur(snap.Elapsed), snap.Online, snap.TotalRequests, snap.TotalLoops, snap.TotalErrors, rt)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func protoName(p *stats.ProtoSnapshot) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Key.String()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-2] + ".."
}

func fmtDur(d time.Duration) string {
	switch {
	case d <= 0:
		return "-"
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return d.Round(time.Second).String()
	}
}

func fmtBytes(n int64) string {
	switch {
	case n < 0:
		return "N/A"
	case n >= 1<<30:
		return fmt.Sprintf("%.2fGB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.0fKB", float64(n)/(1<<10))
	}
}

func fmtCPU(cores float64) string {
	if cores < 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.2f核", cores)
}

func fmtCount(n int64) string {
	if n < 0 {
		return "N/A"
	}
	return strconv.FormatInt(n, 10)
}

// sortModules 保证 core 模块排最后（面板可读性）。
func sortModules(ms []*stats.ModuleSnapshot) {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].Name == "core" {
			return false
		}
		if ms[j].Name == "core" {
			return true
		}
		return ms[i].Name < ms[j].Name
	})
}
