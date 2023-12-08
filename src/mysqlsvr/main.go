package main

import (
	"flag"
	"github.com/Iori372552686/GoOne/common/misc"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/application"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/cmd_handler"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/config"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
)

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

var svrConfFile = flag.String("svr_conf", "./msyqlsvr_conf.json", "app conf file")

type MysqlSvrImpl struct {
}

func (a *MysqlSvrImpl) OnInit() error {
	err := a.OnReload()
	if err != nil {
		logger.Fatalf("Failed to load config | %v", err)
		return err
	}

	for _, ins := range config.SvrCfg.DbInstances {
		err = globals.MysqlMgr.AddInstance(ins.InstanceId,
			ins.Ip,
			ins.Port,
			ins.User,
			ins.Password,
			ins.Schema,
		)
		if err != nil {
			logger.Errorf("failed to add mysql instance | %v", err)
			return err
		}
	}

	err = router.InitAndRun(config.SvrCfg.SelfBusId,
		onRecvSSPacket,
		config.SvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		config.SvrCfg.ZKAddr,
	)
	if err != nil {
		logger.Fatalf("Failed to initialize router | %v", err)
		return err
	}

	cmd_handler.RegCmd()
	globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)

	logger.Infof("mysqlsvr init success")
	return nil
}

func (a *MysqlSvrImpl) OnReload() error {
	err := marshal.LoadJson(*svrConfFile, &config.SvrCfg)
	if err != nil {
		logger.Fatalf("failed to load svr conf | %s", err)
		return err
	}
	return nil
}

func (a *MysqlSvrImpl) OnProc() bool { // return: isIdle
	return true
}

func (a *MysqlSvrImpl) OnTick(lastMs, nowMs int64) {
}

func (a *MysqlSvrImpl) OnExit() {
	globals.MysqlMgr.Destroy()
}

func main() {
	flag.Parse()
	defer logger.Flush()

	application.Init(&MysqlSvrImpl{})
	application.Run()
}
