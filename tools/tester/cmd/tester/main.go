// tester 回归测试入口（独立客户端进程）。
//
// 连接外部已启动的 GoOne 服务器，按配置并发拉起模拟玩家并执行业务用例集，
// 全部通过则退出码 0，否则退出码 1。
//
// 用法：
//
//	tester.exe -config ./tools/tester/tester.toml
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/Iori372552686/GoOne/tools/tester/app/engine"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"

	// 注册全部业务测试组件
	_ "github.com/Iori372552686/GoOne/tools/tester/app/component/login"
	_ "github.com/Iori372552686/GoOne/tools/tester/app/component/room"
)

func main() {
	configFlag := flag.String("config", "", "配置文件路径（默认 ./tools/tester/tester.toml）")
	flag.Parse()

	cfgPath := testcfg.ConfigPath(*configFlag, "./tools/tester")
	cfg, err := testcfg.Load(cfgPath)
	if err != nil {
		log.Fatalf("[Tester] load config: %v", err)
	}
	if cfg.Run.Mode != testcfg.RunModeRegression {
		log.Printf("[Tester] warning: [run].mode = %q, 本入口强制以 regression 模式运行", cfg.Run.Mode)
	}

	// 可达性探测：若网关不可达（CI 未起服务器等场景），优雅跳过而非阻塞/panic。
	// 可用环境变量 TESTER_SKIP_E2E=1 强制跳过。
	if testcfg.SkipE2EFromEnv() {
		log.Printf("[Tester] TESTER_SKIP_E2E=1，跳过端到端回归")
		os.Exit(0)
	}
	if !cfg.Server.GatewayReachable(0) {
		log.Printf("[Tester] 网关 %s 不可达，跳过端到端回归（需先启动 connsvr）", cfg.Server.GatewayHostPort())
		os.Exit(0)
	}

	modules := cfg.EnabledModules()
	eng := engine.NewEngine(cfg, modules, nil, 1)

	allPassed := eng.Run(context.Background())

	if allPassed {
		log.Println("[Tester] ALL PASSED")
		os.Exit(0)
	}
	log.Println("[Tester] SOME FAILED")
	os.Exit(1)
}
