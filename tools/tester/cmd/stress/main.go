// stress 全流程压力测试入口（独立客户端进程）。
//
// 连接外部已启动的服务器（配置 [server]），模拟 N 个玩家登录后按随机/固定顺序
// 循环执行业务操作；运行期间支持终端命令干预（add/remove/pause/stop 等）；
// 结束或中断后生成 Markdown 测试报告，并周期性存档服务器 pprof profile。
//
// 用法：
//
//	stress.exe -config ./etc/stress/tester.toml
//	（缺省读取 ./tools/tester/stress.toml，支持 GOONE_TESTER_* 环境变量覆盖）
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/internal/console"
	"github.com/Iori372552686/GoOne/tools/tester/internal/pprofcollect"
	"github.com/Iori372552686/GoOne/tools/tester/internal/report"
	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	"github.com/Iori372552686/GoOne/tools/tester/stress"

	// 注册全部业务测试组件
	_ "github.com/Iori372552686/GoOne/tools/tester/app/component/login"
	_ "github.com/Iori372552686/GoOne/tools/tester/app/component/room"
)

func main() {
	configFlag := flag.String("config", "", "配置文件路径（默认 ./tools/tester/stress.toml）")
	flag.Parse()

	cfgPath := testcfg.ConfigPath(*configFlag, "./tools/tester")
	cfg, err := testcfg.Load(cfgPath)
	if err != nil {
		log.Fatalf("[Stress] load config: %v", err)
	}
	if cfg.Run.Mode != testcfg.RunModeStress {
		log.Printf("[Stress] warning: [run].mode = %q, 本入口强制以 stress 模式运行", cfg.Run.Mode)
	}

	// 可达性探测：若网关不可达（CI 未起服务器等场景），优雅跳过而非阻塞/panic。
	// 可用环境变量 TESTER_SKIP_E2E=1 强制跳过。
	if testcfg.SkipE2EFromEnv() {
		log.Printf("[Stress] TESTER_SKIP_E2E=1，跳过压测")
		os.Exit(0)
	}
	if !cfg.Server.GatewayReachable(0) {
		log.Printf("[Stress] 网关 %s 不可达，跳过压测（需先启动 connsvr）", cfg.Server.GatewayHostPort())
		os.Exit(0)
	}

	collector := stats.NewCollector()

	ctl, err := stress.NewController(cfg, collector)
	if err != nil {
		log.Fatalf("[Stress] %v", err)
	}

	// pprof 采集
	runStamp := collector.StartAt().Format("20060102_150405")
	profileDir := ""
	if cfg.Collect.ProfileIntervalParsed() > 0 {
		profileDir = filepath.Join(cfg.Collect.ReportDir, "profiles", runStamp)
	}
	pcollector := pprofcollect.New(cfg.Server.PprofBaseURL(), profileDir)
	go pcollector.Run(ctl.Ctx(), cfg.Collect.SampleIntervalParsed(), cfg.Collect.ProfileIntervalParsed())

	// 指标时间序列采样
	go func() {
		ticker := time.NewTicker(cfg.Collect.SampleIntervalParsed())
		defer ticker.Stop()
		for {
			select {
			case <-ctl.Ctx().Done():
				return
			case <-ticker.C:
				collector.TakeSample(pcollector.Latest())
			}
		}
	}()

	// Ctrl+C 优雅停止
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		ctl.Stop("中断（Ctrl+C）")
		// 第二次 Ctrl+C 强制退出
		<-sigCh
		os.Exit(1)
	}()

	// 终端面板 + 命令
	cons := console.New(ctl, collector)
	go cons.Run(ctl.Ctx())

	gateway := cfg.Server.TcpAddr()
	if cfg.Server.Transport == "ws" {
		gateway = cfg.Server.WsURL()
	}
	log.Printf("[Stress] gateway=%s pprof=%s players=%d flow=%s loop=%v duration=%s modules=%v",
		gateway, cfg.Server.PprofBaseURL(),
		cfg.Player.Players, cfg.Stress.Flow, cfg.Stress.Loop, cfg.Stress.Duration, ctl.Modules())

	ctl.Start()
	<-ctl.Done()

	// 收尾采样一次，保证报告含最终数据点
	collector.TakeSample(pcollector.Latest())

	// 生成报告
	meta := report.Meta{
		StopReason: ctl.StopReason(),
		Mode:       cfg.Stress.Flow,
		Loop:       cfg.Stress.Loop,
		Players:    ctl.MaxSlot(),
		StartUID:   cfg.Player.StartUID,
		GatewayURL: gateway,
		PprofURL:   cfg.Server.PprofBaseURL(),
		ProfileDir: profileDir,
		Modules:    ctl.Modules(),
	}
	path, err := report.Write(cfg.Collect.ReportDir, meta, collector.Snapshot())
	if err != nil {
		log.Fatalf("[Stress] write report: %v", err)
	}

	fmt.Println()
	log.Printf("[Stress] 测试结束（%s），报告已生成: %s", ctl.StopReason(), path)
}
