package main

import (
	"GoOne/common/misc"
	"GoOne/common/module/application"
	"GoOne/lib/logger"
	"GoOne/lib/marshal"
	"GoOne/lib/router"
	"GoOne/lib/sharedstruct"
	g1_protocol "GoOne/protobuf/protocol"
	"GoOne/src/connsvr/cmd_handler"
	"GoOne/src/connsvr/config"
	"GoOne/src/connsvr/globals"
	"flag"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"

)

var svrConfFile = flag.String("svr_conf", "./connsvr_conf.json", "app conf file")

func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	if misc.IsClientCmd(packet.Header.Cmd) {
		csPacketHeader := sharedstruct.CSPacketHeader{
			Uid: packet.Header.Uid,
			Cmd: packet.Header.Cmd,
			BodyLen: packet.Header.BodyLen,
		}
		globals.ConnTcpSvr.SendByUid(packet.Header.Uid, csPacketHeader.ToBytes(), packet.Body)
	} else if packet.Header.Cmd == uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ) {
			onSSPacketConnKickout(packet)
	} else {
		globals.TransMgr.ProcessSSPacket(packet)
		packet = nil  // packet所有权转交给transmgr，后面不能再用packet（包括data）
	}
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
	globals.ConnTcpSvr.KickByRemoteAddr(packet.Header.Uid, req.Reason, req.RemoteAddr)
}


type ConnSvrImpl struct {
}

func (a *ConnSvrImpl) OnInit() error {
	err := a.OnReload()
	if err != nil {
		logger.Fatalf("Failed to load config | %v", err)
		return err
	}

	err = router.InitAndRun(config.SvrCfg.SelfBusId,
		onRecvSSPacket,
		config.SvrCfg.RabbitMQAddr,
		misc.ServerRouteRules,
		config.SvrCfg.ZKAddr,
	)
	if err != nil {
		logger.Errorf("Failed to initialize Router | %v", err)
		return err
	}

	cmd_handler.RegCmd()
	globals.TransMgr.InitAndRun(misc.MaxTransNumber, false, 0)

	err = globals.ConnTcpSvr.InitAndRun("0.0.0.0", config.SvrCfg.ListenPort)
	if err != nil {
		logger.Errorf("Failed to initialize TcpServer | %v", err)
		return err
	}
	globals.ConnTcpSvr.TcpReadTimeout = misc.ClientExpiryThreshold * 2

	return nil
}

func (a *ConnSvrImpl) OnReload() error {
	err := marshal.LoadJson(*svrConfFile, &config.SvrCfg)
	if err != nil {
		logger.Fatalf("Failed to load server config | %s", err)
		return err
	}
	logger.Infof("svr_conf: %#v", config.SvrCfg)

	return nil
}

func (a *ConnSvrImpl) OnProc() bool {
	return true
}

func (a *ConnSvrImpl) OnTick(lastMs, nowMs int64) {
}

func (a *ConnSvrImpl) OnExit() {
}

func main() {
	flag.Parse()
	defer logger.Flush()

	application.Init(&ConnSvrImpl{})
	application.Run()
}

