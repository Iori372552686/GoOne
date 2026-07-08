package router

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus"
	"github.com/Iori372552686/GoOne/lib/service/svrinstmgr"
	"github.com/Iori372552686/GoOne/module/misc"

	"github.com/golang/protobuf/proto"
)

// router
// . 主要处理服务器之间的消息收发
// . 使用bus作为底层的消息转输
// 要求：需要保证协程安全

// -------------------------------- public --------------------------------

func SelfBusId() uint32 {
	if router.busImpl == nil {
		return 0
	}
	return router.busImpl.SelfBusId()
}

func SelfSvrType() uint32 {
	return (SelfBusId() >> 8) & 0xff
}

type CbOnRecvSSPacket func(*sharedstruct.SSPacket) // frameMsg的所有权，归回调函数

// cb CbOnRecvSSPacket将由底层(bus)协程调用
func InitAndRun(selfBusId string, cb CbOnRecvSSPacket, rabbitmqAddr string,
	routeRules map[uint32]uint32, zookeeperAddr string) error {
	router.beginShutdownOnce = sync.Once{}
	router.closeOnce = sync.Once{}
	router.shuttingDown.Store(false)
	err := severInstanceMgr.InitAndRun(selfBusId, routeRules, zookeeperAddr)
	if err != nil {
		return err
	}

	router.cbOnRecvSSPacket = cb
	busImpl, err := bus.CreateBus(bus.IpStringToInt(selfBusId), onRecvBusMsg, rabbitmqAddr)
	if err != nil {
		return fmt.Errorf("failed to create bus implement: %w", err)
	}
	if busImpl == nil {
		return errors.New("failed to create bus implement")
	}
	router.busImpl = busImpl
	return nil
}

func BeginShutdown() {
	router.beginShutdownOnce.Do(func() {
		router.shuttingDown.Store(true)
		severInstanceMgr.Close()
	})
}

func Close() error {
	var closeErr error
	router.closeOnce.Do(func() {
		BeginShutdown()
		if router.busImpl != nil {
			router.busImpl.SetReceiver(nil)
			if err := router.busImpl.Close(); err != nil {
				closeErr = err
			}
		}
		router.busImpl = nil
		router.cbOnRecvSSPacket = nil
	})
	return closeErr
}

type AdminSnapshot struct {
	Initialized  bool
	SelfBusID    uint32
	ShuttingDown bool
	BusHealthy   bool
}

func Snapshot() AdminSnapshot {
	return AdminSnapshot{
		Initialized:  router.busImpl != nil,
		SelfBusID:    SelfBusId(),
		ShuttingDown: router.shuttingDown.Load(),
		BusHealthy:   BusHealthy(),
	}
}

// BusHealthy reports whether the underlying bus connection is currently up.
func BusHealthy() bool {
	b := router.busImpl
	return b != nil && b.Healthy()
}

// ReadyCheck is a bootstrap-compatible readiness probe: it fails while the
// router is shutting down or the bus connection is down, so /readyz flips to
// 503 during MQ outages instead of silently accepting traffic.
func ReadyCheck() error {
	if router.shuttingDown.Load() {
		return errors.New("router is shutting down")
	}
	if !BusHealthy() {
		return errors.New("bus is not connected")
	}
	return nil
}

// 最终通过bus发消息的地方（其他都是易用性封装）
func SendMsg(packetHeader *sharedstruct.SSPacketHeader, packetBody []byte) error {
	//logger.Infof("Send bus cmd: %v | %v", g1_protocol.CMD(packetHeader.Cmd), packetHeader)
	if router.busImpl != nil && packetHeader.DstBusID == router.busImpl.SelfBusId() {
		finish := beginRouterObserve("send", "local", packetHeader.Cmd)
		packet := &sharedstruct.SSPacket{
			Header: *packetHeader,
		}
		if len(packetBody) > 0 {
			packet.Body = make([]byte, len(packetBody))
			copy(packet.Body, packetBody)
		}
		if router.cbOnRecvSSPacket != nil {
			router.cbOnRecvSSPacket(packet)
		}
		finish(len(packetBody), nil)
		return nil
	}
	if router.busImpl == nil {
		return errors.New("router bus is not initialized")
	}

	finish := beginRouterObserve("send", "bus", packetHeader.Cmd)
	// 头编码到栈上数组：IBus.Send 会同步把 data1 拷贝进自己的帧缓冲，
	// 不会保留引用，因此这里无需堆分配。
	var headerBuf [54]byte
	_ = packetHeader.To(headerBuf[:])
	err := router.busImpl.Send(packetHeader.DstBusID, headerBuf[:], packetBody)
	finish(len(packetBody), err)
	if err != nil {
		e := fmt.Sprintf("failed to send bus message {header:%#v, bodyLen:%v} | %v",
			packetHeader, len(packetBody), err)
		logger.Errorf(e)
		return errors.New(e)
	}
	return nil
}

func SendPbMsg(packetHeader *sharedstruct.SSPacketHeader, pbMsg proto.Message) error {
	packetBody, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	packetHeader.BodyLen = uint32(len(packetBody))
	if logger.DebugEnabled() {
		logger.CmdDebugf(packetHeader.Cmd,
			"SendPbMsg {cmd:%v, uid:%v, rid:%v, srcBusId:%v, dstBusId:%v, bodyLen:%d, msgType:%s}",
			packetHeader.Cmd, packetHeader.Uid, packetHeader.RouterID, packetHeader.SrcBusID, packetHeader.DstBusID,
			len(packetBody), protoMessageType(pbMsg))
	}
	return SendMsg(packetHeader, packetBody)
}

func SendMsgByBusId(busId uint32, routerKey, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	if busId == 0 {
		logger.Errorf("server instance is 0, fail to send {busId: %v, uid: %v, cmd: %X}", busId, uid, cmd)
		return errors.New("server instance is 0, fail to send")
	}

	packetHeader := sharedstruct.SSPacketHeader{
		SrcBusID:   SelfBusId(),
		DstBusID:   busId,
		SrcTransID: srcTransId,
		DstTransID: 0,
		Uid:        uid,
		Cmd:        uint32(cmd),
		RouterID:   routerKey,
		BodyLen:    uint32(len(data)),
		CmdSeq:     sendSeq,
		Zone:       zone,
	}

	return SendMsg(&packetHeader, data)
}

func SendPbMsgByBusId(busId uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	if logger.DebugEnabled() {
		logger.CmdDebugf(uint32(cmd),
			"SendPbMsgByBusId {dstBusId:%v, uid:%v, zone:%v, cmd:%v, bodyLen:%d, msgType:%s}",
			busId, uid, zone, uint32(cmd), len(data), protoMessageType(pbMsg))
	}
	return SendMsgByBusId(busId, 0, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendPbMsgByBusIdSimple(busId uint32, uid uint64, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return SendPbMsgByBusId(busId, uid, 1, cmd, 0, 0, pbMsg)
}

func SendMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	dstBusId, routerKey := severInstanceMgr.GetSvrInsBySvrType(svrType, zone, uid, routerId)
	if dstBusId == 0 {
		logger.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %v}", svrType, uid, cmd)
		return errors.New("cannot get a server instance to send")
	}

	return SendMsgByBusId(dstBusId, routerKey, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendMsgByConn(uid, routerId uint64, zone, cmd uint32, srcTransId uint32, data []byte, ip, port uint32) error {
	svrType := misc.ServerTypeInCmd(cmd)
	dstBusId, routerKey := severInstanceMgr.GetSvrInsBySvrType(svrType, zone, uid, routerId)
	if dstBusId == 0 {
		logger.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %v}", svrType, uid, cmd)
		return errors.New("cannot get a server instance to send")
	}

	packetHeader := sharedstruct.SSPacketHeader{
		SrcBusID:   SelfBusId(),
		DstBusID:   dstBusId,
		SrcTransID: srcTransId,
		DstTransID: 0,
		Uid:        uid,
		Cmd:        cmd,
		RouterID:   routerKey,
		BodyLen:    uint32(len(data)),
		Ip:         ip,
		Flag:       port,
		Zone:       zone,
	}

	return SendMsg(&packetHeader, data)
}

func SendPbMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	//logger.Debugf("SendPbMsgBySvrType: %#v", pbMsg.String())
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	return SendMsgBySvrType(svrType, routerId, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendPbMsgBySvrTypeSimple(svrType uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return SendPbMsgBySvrType(svrType, uid, uid, zone, cmd, 0, 0, pbMsg)
}

func SendPbMsgByRouter(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return SendPbMsgBySvrType(svrType, routerId, uid, zone, cmd, 0, 0, pbMsg)
}

func BroadcastMsgByServerType(svrType uint32, uid uint64, cmd g1_protocol.CMD, sendSeq uint16, data []byte) error {
	instances := severInstanceMgr.GetAllSvrInsBySvrType(svrType)
	if len(instances) == 0 {
		return fmt.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %X}", svrType, uid, cmd)
	}

	for _, inst := range instances {
		SendMsgByBusId(inst, 0, uid, 1, cmd, sendSeq, 0, data)
	}

	return nil
}

func BroadcastPbMsgByServerType(svrType uint32, uid uint64, cmd g1_protocol.CMD, sendSeq uint16, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	if logger.DebugEnabled() {
		logger.CmdDebugf(uint32(cmd),
			"BroadcastPbMsgByServerType {svrType:%v, uid:%v, cmd:%v, bodyLen:%d, msgType:%s}",
			svrType, uid, uint32(cmd), len(data), protoMessageType(pbMsg))
	}
	return BroadcastMsgByServerType(svrType, uid, cmd, sendSeq, data)
}

func SendMsgBack(originalHeader sharedstruct.SSPacketHeader, srcTransId uint32, pbMsg proto.Message) {
	originalHeader.DstBusID = originalHeader.SrcBusID
	originalHeader.SrcBusID = SelfBusId()
	originalHeader.DstTransID = originalHeader.SrcTransID
	originalHeader.SrcTransID = srcTransId
	originalHeader.Cmd = originalHeader.Cmd + 1
	SendPbMsg(&originalHeader, pbMsg)
}

// SendMsgBackWithCmd is the same as SendMsgBack, but allows overriding the response cmd.
// This is used by IDL-driven ssrpc wrappers when cmd_resp != cmd+1.
//
// Note: CmdSeq is kept as-is (matches request CmdSeq), to preserve Transaction.waitRsp semantics.
func SendMsgBackWithCmd(originalHeader sharedstruct.SSPacketHeader, srcTransId uint32, cmd g1_protocol.CMD, pbMsg proto.Message) {
	originalHeader.DstBusID = originalHeader.SrcBusID
	originalHeader.SrcBusID = SelfBusId()
	originalHeader.DstTransID = originalHeader.SrcTransID
	originalHeader.SrcTransID = srcTransId
	originalHeader.Cmd = uint32(cmd)
	SendPbMsg(&originalHeader, pbMsg)
}

// -------------------------------- private --------------------------------

var severInstanceMgr svrinstmgr.ServerInstanceMgr

var router struct {
	busImpl           bus.IBus
	cbOnRecvSSPacket  CbOnRecvSSPacket
	beginShutdownOnce sync.Once
	closeOnce         sync.Once
	shuttingDown      atomic.Bool
}

func protoMessageType(msg proto.Message) string {
	if msg == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", msg)
}

func onRecvBusMsg(srcBusId uint32, data []byte) error {
	//logger.Debugf("Received message bus:%v, len: %v\n", bus.IpIntToString(srcBusId), len(data))
	if len(data) < sharedstruct.ByteLenOfSSPacketHeader() {
		err := fmt.Errorf("bus message is too short {len:%v, expect:%v}", len(data), sharedstruct.ByteLenOfSSPacketHeader())
		observeRouterInvalidReceive("invalid_short_packet", err, len(data))
		return err
	}

	packet := new(sharedstruct.SSPacket)
	packet.Header.From(data)
	packet.Body = data[sharedstruct.ByteLenOfSSPacketHeader():]
	if logger.DebugEnabled() {
		logger.CmdDebugf(packet.Header.Cmd, "[uid: %d] Received bus message: %+v", packet.Header.Uid, packet.Header)
	}
	finish := beginRouterObserve("receive", "bus", packet.Header.Cmd)
	if router.cbOnRecvSSPacket != nil {
		router.cbOnRecvSSPacket(packet)
	}
	finish(len(packet.Body), nil)

	return nil
}
