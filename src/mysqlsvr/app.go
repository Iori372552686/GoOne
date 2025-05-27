package main

import (
	"runtime"

	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/cmd_handler"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/globals"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
)

type MysqlSvrImpl struct {
}

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	globals.TransMgr.ProcessSSPacket(packet)
	packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
}

func (a *MysqlSvrImpl) OnInit() error {
	//-- set sys args
	runtime.GOMAXPROCS(runtime.NumCPU() + 1)

	//-- load cfg
	err := a.OnReload()
	if err != nil {
		logger.Fatalf("Failed to load config | %v", err)
		return err
	}

	// init logger
	if _, err = logger.InitLogger(gconf.MySqlSvrCfg.MySqlSvr.LogDir, gconf.MySqlSvrCfg.MySqlSvr.LogLevel, "mysqlsvr"); err != nil {
		return err
	}

	//-- init ormMgr
	err = globals.OrmMgr.InitAndRun(gconf.MySqlSvrCfg.OrmConf, manager.GetTables()...)
	if err != nil {
		logger.Errorf("OrmMgr InitAndRun error !! err | %v", err)
		return err
	}

	//-- init orm cache in some table
	//cacher := xorm.NewLRUCacher(xorm.NewMemoryStore(), misc.MaxOrmLruCacheLimitNum)
	//err = globals.OrmMgr.GetOrmEngine().MapCacher(&define.ActivityRoleInfo{}, cacher)
	//err1 := globals.OrmMgr.GetOrmEngine().MapCacher(&define.ConvertRoleInfo{}, cacher)
	//if err != nil || err1 != nil {
	//	logger.Errorf("init orm cache error !! err | %v  ,err1 | %v", err, err1)
	//	return err
	//}

	err = router.InitAndRun(gconf.MySqlSvrCfg.SelfBusId,
		onRecvSSPacket,
		gconf.MySqlSvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		gconf.MySqlSvrCfg.ZKAddr,
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
	err := marshal.LoadConfFile(*gconf.SvrConfFile, &gconf.MySqlSvrCfg)
	if err != nil {
		logger.Fatalf("failed to load svr config | %s", err)
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
	manager.Close()
	globals.MysqlMgr.Destroy()
}
