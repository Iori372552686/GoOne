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
