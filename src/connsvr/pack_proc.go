package main

import (
	"net"
	"strconv"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
	"github.com/Iori372552686/GoOne/src/connsvr/login"
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
)

// proc WebSocket packet
func onWebSocketPacket(conn net.Conn, data []byte) {
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	logger.Debugf("onWebSocketPacket: {dataLen: %v, headerLen: %v, remoteAddr: %v}\n",
		len(data), headerLen, conn.RemoteAddr().String())

	packetHeader := sharedstruct.CSPacketHeader{}
	if len(data) < packetHeader.Size() {
		logger.Errorf("Received datalen < packetHeader, packet is invalid")
		return
	}

	packetHeader.From(data)
	packetBody := data[headerLen:]
	logger.CmdDebugf(packetHeader.Cmd, "[uid: %d] Received client packet: %#v", packetHeader.Uid, packetHeader)

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}

	// 实现帐号认证系统之前，由client决定uid
	uid := packetHeader.Uid
	var client *net_mgr.Client
	if uid > 0 {
		client = globals.ConnWsSvr.GetClientByUid(uid)
		if client == nil {
			logger.Errorf("Cannot find conn by uid: %v", uid)
			return
		}

		//前期简单测试，后期改为严谨通过rebind 与账号服验证后更新conn
		if client.Conn != conn {
			globals.ConnWsSvr.UpdateClientByUid(conn, uid, client.Zone) // update conn
		}

	} else {
		if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
			req := &g1_protocol.LoginReq{}

			startTime := datetime.NowMs()
			err := proto.Unmarshal(packetBody, req)
			if err != nil {
				logger.Errorf(" Fail to unmarshal LoginReq | %v ", err)
				return
			}

			if req.GetAccount() == "" || req.GetChannelId() == 0 {
				logger.Errorf(" LoginReq  -- > account or ChannelId   error !!! ")
				return
			}

			ret, accUid := login.OnCheckAuthByAccSvr(req.Account, req.Token, req.ChannelId, req.LoginType)
			duration := datetime.NowMs() - startTime
			logger.Infof("LoginReq  -- > CheckAuthByAccSvr spent ms : %s  | ret=%v ", strconv.FormatInt(duration, 10), ret)
			if !ret {
				// 后续返回错误给前端
				return
			}

			uid = accUid
			zone := uint32(1)                                             //根据channelid 从配置获取 todo
			client = globals.ConnWsSvr.UpdateClientByUid(conn, uid, zone) // update conn
		} else {
			logger.Errorf("Cannot find conn by uid , need login!!: %v", uid)
			return
		}
	}

	router.SendMsgByConn(uid, uid, client.Zone, packetHeader.Cmd, 0, packetBody, client.Ip, client.Port)
}

// proc tcp packet
func onTcpPacket(conn net.Conn, data []byte) {
	startTime := datetime.NowMs()
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	logger.Debugf("OnTcpPacket: {dataLen: %v, headerLen: %v, remoteAddr: %v}\n",
		len(data), headerLen, conn.RemoteAddr())

	packetHeader := sharedstruct.CSPacketHeader{}
	if len(data) < packetHeader.Size() {
		logger.Errorf("Received datalen < packetHeader, packet is invalid")
		return
	}

	packetHeader.From(data)
	packetBody := data[headerLen:]
	logger.CmdDebugf(packetHeader.Cmd, "[uid: %d] Received client packet: %#v", packetHeader.Uid, packetHeader)

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}

	// 实现帐号认证系统之前，由client决定uid
	uid := packetHeader.Uid
	var client *net_mgr.Client
	if uid > 0 {
		client = globals.ConnWsSvr.GetClientByUid(uid)
		if client == nil {
			logger.Errorf("Cannot find conn by uid: %v", uid)
			return
		}

		//前期简单测试，后期改为严谨通过rebind 与账号服验证后更新conn
		if client.Conn != conn {
			globals.ConnTcpSvr.UpdateClientByUid(conn, uid, client.Zone) // update conn
		}

	} else {
		if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
			req := &g1_protocol.LoginReq{}

			err := proto.Unmarshal(packetBody, req)
			if err != nil {
				logger.Errorf(" Fail to unmarshal LoginReq | %v ", err)
				return
			}

			if req.GetAccount() == "" || req.GetChannelId() == 0 {
				logger.Errorf(" LoginReq  -- > account or ChannelId   error !!! ")
				return
			}

			ret, accUid := login.OnCheckAuthByAccSvr(req.Account, req.Token, req.ChannelId, req.LoginType)
			duration := datetime.NowMs() - startTime
			logger.Infof("LoginReq  -- > CheckAuthByAccSvr spent ms : %s", strconv.FormatInt(duration, 10))
			if !ret {
				// 后续返回错误给前端
				return
			}

			uid = accUid
			zone := uint32(1)                                              //根据channelid 从配置获取 todo
			client = globals.ConnTcpSvr.UpdateClientByUid(conn, uid, zone) // update conn
		} else {
			logger.Errorf("Cannot find conn by uid , need login!!: %v", uid)
			return
		}
	}

	router.SendMsgByConn(uid, uid, 0, packetHeader.Cmd, 0, packetBody, client.Ip, client.Port)
}

// busMsg proc cb func
func onRecvSSPacket(packet *sharedstruct.SSPacket) {
	if misc.IsClientCmd(packet.Header.Cmd) {
		csPacketHeader := sharedstruct.CSPacketHeader{
			Uid:     packet.Header.Uid,
			Cmd:     packet.Header.Cmd,
			BodyLen: packet.Header.BodyLen,
		}
		//globals.ConnTcpSvr.SendByUid(packet.Header.Uid, csPacketHeader.ToBytes(), packet.Body)
		globals.ConnWsSvr.SendByUid(packet.Header.Uid, csPacketHeader.ToBytes(), packet.Body)
	} else if packet.Header.Cmd == uint32(g1_protocol.CMD_CONN_KICK_OUT_REQ) {
		onSSPacketConnKickout(packet)
	} else {
		globals.TransMgr.ProcessSSPacket(packet)
		packet = nil // packet所有权转交给transmgr，后面不能再用packet（包括data）
	}
}

// conn kickout
func onSSPacketConnKickout(packet *sharedstruct.SSPacket) {
	logger.Infof("onSSPacketScKickout {header:%#v}", packet.Header)
	req := g1_protocol.ConnKickOutReq{}
	err := proto.Unmarshal(packet.Body, &req)
	if err != nil {
		logger.Warningf("Fail to unmarshal req | %v", err)
		return
	}
	logger.Debugf("Received a req: %#v", req)

	//globals.ConnTcpSvr.KickByRemoteAddr(packet.Header.Uid, req.Reason, req.RemoteAddr)
	globals.ConnWsSvr.KickByRemoteAddr(packet.Header.Uid, req.Reason, req.RemoteAddr)
}
