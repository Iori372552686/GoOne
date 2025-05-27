package main

import (
	"runtime"

	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/infosvr/cmd_handler"
	"github.com/Iori372552686/GoOne/src/infosvr/globals"
)

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

type InfoSvrImpl struct {
}

func (a *InfoSvrImpl) OnInit() error {
	//-- set sys args
	runtime.GOMAXPROCS(runtime.NumCPU() + 1)

	//-- load cfg
	err := a.OnReload()
	if err != nil {
		logger.Errorf("Failed to load config | %v", err)
		return err
	}

	// init zap logger
	if _, err = logger.InitLogger(gconf.InfoSvrCfg.InfoSvr.LogDir, gconf.InfoSvrCfg.InfoSvr.LogLevel, "infosvr"); err != nil {
		return err
	}

	globals.InfoMgr.RedisMgr.InitAndRun(gconf.InfoSvrCfg.DbInstances)

	err = router.InitAndRun(gconf.InfoSvrCfg.SelfBusId,
		onRecvSSPacket,
		gconf.InfoSvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		gconf.InfoSvrCfg.ZKAddr,
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
	err := marshal.LoadConfFile(*gconf.SvrConfFile, &gconf.InfoSvrCfg)
	if err != nil {
		logger.Errorf("failed to load svr config | %s", err)
		return err
	}
	return nil
}

/**
* @Description:  proc
* @return: bool
* @Author: Iori
* @Date: 2022-04-27 21:05:01
**/
func (self *InfoSvrImpl) OnProc() bool {
	// mainloop  proc
	return true
}

/**
* @Description: mainloop tick
* @param: lastMs
* @param: nowMs
* @Author: Iori
* @Date: 2022-04-27 21:04:53
**/
func (self *InfoSvrImpl) OnTick(lastMs, nowMs int64) {
}

/**
* @Description: main exit
* @Author: Iori
* @Date: 2022-04-27 21:05:07
**/
func (self *InfoSvrImpl) OnExit() {
	// game exit todo something
	logger.Flush()
	logger.Infof("service exit, right now !")
	logger.Infof("================== MainSvrImpl Stop =========================")
}
