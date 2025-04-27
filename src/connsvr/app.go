package main

import (
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/misc"

	"github.com/Iori372552686/GoOne/lib/util/marshal"

	"github.com/Iori372552686/GoOne/src/connsvr/cmd_handler"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"

	"runtime"
)

// gameSvr  struct
type AppSvrImpl struct{}

/**
* @Description:init
* @return: error
* @Author: Iori
* @Date: 2022-04-27 21:04:30
**/
func (self *AppSvrImpl) OnInit() error {
	//-- set sys args
	runtime.GOMAXPROCS(runtime.NumCPU() + 1)

	//-- load cfg
	err := self.OnReload()
	if err != nil {
		logger.Errorf("Failed to load config | %v", err)
		return err
	}

	// init zap logger
	if err = logger.InitLog(gconf.ConnSvrCfg.ConnSvr.LogDir, gconf.ConnSvrCfg.ConnSvr.LogLevel, "connsvr"); err != nil {
		return err
	}

	err = router.InitAndRun(gconf.ConnSvrCfg.SelfBusId,
		onRecvSSPacket,
		gconf.ConnSvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		gconf.ConnSvrCfg.ZKAddr,
	)
	if err != nil {
		logger.Errorf("Failed to initialize Router | %v", err)
		return err
	}

	//-- init Sign Mgr
	globals.SignMgr.InitAndRun(gconf.ConnSvrCfg.HTTPSigns)
	//-- init RestApi mgr
	globals.RestMgr.Init(gconf.ConnSvrCfg.RestApiConf, globals.SignMgr)

	cmd_handler.RegCmd()
	globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)

	err = globals.ConnTcpSvr.CreateTcpServer("", gconf.ConnSvrCfg.ListenPort+1, onTcpPacket)
	if err != nil {
		logger.Errorf("Failed to initialize TcpServer | %v", err)
		return err
	}

	return globals.ConnWsSvr.CreateWebSocketServer("gin", "debug", gconf.ConnSvrCfg.ListenPort, onWebSocketPacket)
}

/**
* @Description:  reload
* @return: error
* @Author: Iori
* @Date: 2022-04-27 21:04:41
**/
func (self *AppSvrImpl) OnReload() error {
	// load start_conf, game_xlc_cfg_data..
	err := marshal.LoadConfFile(*gconf.SvrConfFile, &gconf.ConnSvrCfg)
	if err != nil {
		logger.Errorf("Failed to load server config | %s", err)
		return err
	}

	logger.Infof("svr_conf: %+v", gconf.ConnSvrCfg)
	return nil
}

/**
* @Description:  proc
* @return: bool
* @Author: Iori
* @Date: 2022-04-27 21:05:01
**/
func (self *AppSvrImpl) OnProc() bool {
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
func (self *AppSvrImpl) OnTick(lastMs, nowMs int64) {
}

/**
* @Description: main exit
* @Author: Iori
* @Date: 2022-04-27 21:05:07
**/
func (self *AppSvrImpl) OnExit() {
	// game exit todo something
	logger.Flush()
	logger.Infof("service exit, right now !")
	logger.Infof("================== AppSvrImpl Stop =========================")
}
