package main

import (
	`flag`
	`fmt`
	`runtime`
	`time`

	`GoOne/common`
	`GoOne/common/misc`
	`GoOne/common/module/application`
	`GoOne/common/module/datetime`
	`GoOne/lib/logger`
	`GoOne/lib/marshal`
	`GoOne/lib/router`
	`GoOne/lib/sharedstruct`
	web `GoOne/lib/web/client`
	webrouters `GoOne/lib/web/routers`
	g1_protocol `GoOne/protobuf/protocol`
	`GoOne/src/connsvr/cmd_handler`
	config `GoOne/src/connsvr/conf`
	`GoOne/src/connsvr/globals`

	`github.com/astaxie/beego`
	`github.com/astaxie/beego/plugins/cors`
	`github.com/beego/i18n`

	`github.com/golang/glog`
	`github.com/golang/protobuf/proto`
)

type ConnSvrImpl struct {}
var svrConfFile = flag.String("svr_conf", "./connsvr_conf.json", "app conf file")


func InitLog() {
	fpath := fmt.Sprintf("/home/bian_game/data/log/MainServer")
	if runtime.GOOS != "linux" {fpath = "logs"}

	fpath = "logs"
	filename := fpath + "/" + time.Now().Add(time.Second).Local().String()[0:10] + ".log"
	isLog, _ := beego.AppConfig.Bool("isLog")
	if isLog {logger.SetWriteFile(filename, isLog)}
	return
}


func onSSPacketConnKickout(packet *sharedstruct.SSPacket) {
	glog.Infof("onSSPacketScKickout {header:%#v}", packet.Header)
	req := g1_protocol.ConnKickOutReq{}
	err := proto.Unmarshal(packet.Body, &req)
	if err != nil {
		glog.Warningf("Fail to unmarshal req | %v", err)
		return
	}
	logger.Debugf("Received a req: %#v", req)

	// globals.ConnTcpSvr.Kick(packet.Header.Uid, req.Reason)
	globals.ClientMgr.KickById(packet.Header.Uid, req.Reason)
}


func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	if common.IsClientCmd(packet.Header.Cmd) {
		csPacketHeader := sharedstruct.CSPacketHeader{
			Uid: packet.Header.Uid,
			Cmd: packet.Header.Cmd,
			BodyLen: packet.Header.BodyLen,
		}
		globals.ClientMgr.SendByUid(packet.Header.Uid, csPacketHeader.ToBytes(), packet.Body)
	} else if packet.Header.Cmd == uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ) {
		onSSPacketConnKickout(packet)
	} else {
		globals.TransMgr.ProcessSSPacket(packet)
		packet = nil
	}
}

func (c ConnSvrImpl) OnInit() error {
	//set Sysctl
	runtime.GOMAXPROCS(runtime.NumCPU())

	//init conf
	//InitLog()
	err := c.OnReload()
	if err != nil {
		logger.Fatalf("Failed to load config | %v", err)
		return err
	}

	//reg RegCmd
	cmd_handler.RegCmd()

	//run WebRouter
	webrouters.InitRouter()

	//run Beego
	beego.InsertFilter("*", beego.BeforeRouter, cors.Allow(&cors.Options{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Access-Control-Allow-Origin", "Access-Control-Allow-Headers", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length", "Access-Control-Allow-Origin", "Access-Control-Allow-Headers", "Content-Type"},
		AllowCredentials: true,
	}))
	beego.AddFuncMap("i18n", i18n.Tr)
//	beego.Run()

	//init rb zk
	_ = router.InitAndRun(config.SvrCfg.SelfBusId,
		onRecvSSPacket,
		config.SvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		config.SvrCfg.ZKAddr,
	)

	//init Redis
	//for _, ins := range config.SvrCfg.DbInstances {
	//	_ = redis.RedisMgr.AddInstance(ins.InstanceId, ins.Ip, ins.Port, ins.Password, 0, ins.IsCluster)
	//}

	//run mgr
	web.InitClientManager()

	//run transMgr
	globals.TransMgr.InitAndRun(100, false, 0)

	logger.Infof("mainsvr init success")
	return nil
}

func (c ConnSvrImpl) OnReload() error {
	err := marshal.LoadJson(*svrConfFile, &config.SvrCfg)
	if err != nil {
		logger.Fatalf("Failed to load server config | %s", err)
		return err
	}
	logger.Infof("svr_conf: %#v", config.SvrCfg)


	return nil
}

func (c ConnSvrImpl) OnProc() bool {
	return true
}

func (c ConnSvrImpl) OnTick(lastMs, nowMs int64) {
	if lastMs / datetime.MS_PER_MINUTE != nowMs / datetime.MS_PER_MINUTE {   // 一分钟调用
		logger.Infof("定时任务，清理超时连接")
		web.ClearTimeoutConnections()
	}
}

func (c ConnSvrImpl) OnExit() {
	//panic("implement me")
}

func main() {
	logger.Infof("======================= app start =========================")
	flag.Parse()
	defer logger.Flush()

	application.Init(&ConnSvrImpl{})
	application.Run()
	logger.Infof("======================= app end =========================")
}

