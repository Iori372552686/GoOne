package main

import (
	"GoOne/lib/api/logger"
	"GoOne/lib/api/sharedstruct"
	"GoOne/lib/service/application"
	"GoOne/lib/util/marshal"

	"GoOne/lib/service/router"
	"flag"

	"GoOne/common/misc"
	"GoOne/src/infosvr/cmd_handler"
	"GoOne/src/infosvr/config"
	"GoOne/src/infosvr/globals"
)

var svrConfFile = flag.String("svr_conf", "./infosvr_conf.json", "app conf file")

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

type InfoSvrImpl struct {
}

func (a *InfoSvrImpl) OnInit() error {
	err := a.OnReload()
	if err != nil {
		logger.Fatalf("Failed to load config | %v", err)
		return err
	}

	for _, ins := range config.SvrCfg.DbInstances {
		_ = globals.InfoMgr.RedisMgr.AddInstance(ins.InstanceId, ins.Ip, int(ins.Port), ins.Password, 0, ins.IsCluster)
	}

	err = router.InitAndRun(config.SvrCfg.SelfBusId,
		onRecvSSPacket,
		config.SvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		config.SvrCfg.ZKAddr,
	)
	if err != nil {
		return err
	}

	cmd_handler.RegCmd()
	globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)

	logger.Infof("infosvr init success")
	return nil
}

func (a *InfoSvrImpl) OnReload() error {
	err := marshal.LoadJson(*svrConfFile, &config.SvrCfg)
	if err != nil {
		logger.Fatalf("failed to load svr conf | %s", err)
		return err
	}
	return nil
}

func (a *InfoSvrImpl) OnProc() bool { // return: isIdle
	return true
}

func (a *InfoSvrImpl) OnTick(lastMs, nowMs int64) {
}

func (a *InfoSvrImpl) OnExit() {
}

func main() {
	flag.Parse()
	defer logger.Flush()

	application.Init(&InfoSvrImpl{})
	application.Run()
}
