// Package router 处理服务器之间的消息收发，使用 bus 作为底层消息传输。
//
// 包级函数操作默认实例（Default()），与历史 API 完全兼容；
// 需要隔离状态的场景（单元测试、同进程多实例）可通过 New() 创建独立 Router。
// 所有方法要求协程安全。
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

type CbOnRecvSSPacket func(*sharedstruct.SSPacket) // frameMsg的所有权，归回调函数

// Router owns one bus connection and its service-instance view.
type Router struct {
	busImpl           bus.IBus
	cbOnRecvSSPacket  CbOnRecvSSPacket
	instanceMgr       svrinstmgr.ServerInstanceMgr
	beginShutdownOnce sync.Once
	closeOnce         sync.Once
	shuttingDown      atomic.Bool
}

// New creates an independent, uninitialized Router.
func New() *Router {
	return &Router{}
}

var defaultRouter = New()

// Default returns the process-wide default Router used by the package-level
// helper functions.
func Default() *Router {
	return defaultRouter
}

// -------------------------------- methods --------------------------------

func (r *Router) SelfBusId() uint32 {
	if r.busImpl == nil {
		return 0
	}
	return r.busImpl.SelfBusId()
}

func (r *Router) SelfSvrType() uint32 {
	return (r.SelfBusId() >> 8) & 0xff
}

// InitAndRun 初始化服务发现并连接 bus。cb 将由底层 bus 协程调用。
func (r *Router) InitAndRun(selfBusId string, cb CbOnRecvSSPacket, busMQAddr string,
	routeRules map[uint32]uint32, registerAddr string) error {
	r.beginShutdownOnce = sync.Once{}
	r.closeOnce = sync.Once{}
	r.shuttingDown.Store(false)
	if err := r.instanceMgr.InitAndRun(selfBusId, routeRules, registerAddr); err != nil {
		return err
	}

	r.cbOnRecvSSPacket = cb
	busImpl, err := bus.CreateBus(bus.IpStringToInt(selfBusId), r.onRecvBusMsg, busMQAddr)
	if err != nil {
		return fmt.Errorf("failed to create bus implement: %w", err)
	}
	if busImpl == nil {
		return errors.New("failed to create bus implement")
	}
	r.busImpl = busImpl
	return nil
}

func (r *Router) BeginShutdown() {
	r.beginShutdownOnce.Do(func() {
		r.shuttingDown.Store(true)
		r.instanceMgr.Close()
	})
}

func (r *Router) Close() error {
	var closeErr error
	r.closeOnce.Do(func() {
		r.BeginShutdown()
		if r.busImpl != nil {
			r.busImpl.SetReceiver(nil)
			if err := r.busImpl.Close(); err != nil {
				closeErr = err
			}
		}
		r.busImpl = nil
		r.cbOnRecvSSPacket = nil
	})
	return closeErr
}

type AdminSnapshot struct {
	Initialized  bool
	SelfBusID    uint32
	ShuttingDown bool
	BusHealthy   bool
}

func (r *Router) Snapshot() AdminSnapshot {
	return AdminSnapshot{
		Initialized:  r.busImpl != nil,
		SelfBusID:    r.SelfBusId(),
		ShuttingDown: r.shuttingDown.Load(),
		BusHealthy:   r.BusHealthy(),
	}
}

// BusHealthy reports whether the underlying bus connection is currently up.
func (r *Router) BusHealthy() bool {
	b := r.busImpl
	return b != nil && b.Healthy()
}

// ReadyCheck is a bootstrap-compatible readiness probe: it fails while the
// router is shutting down or the bus connection is down, so /readyz flips to
// 503 during MQ outages instead of silently accepting traffic.
func (r *Router) ReadyCheck() error {
	if r.shuttingDown.Load() {
		return errors.New("router is shutting down")
	}
	if !r.BusHealthy() {
		return errors.New("bus is not connected")
	}
	return nil
}

// SendMsg 最终通过bus发消息的地方（其他都是易用性封装）
func (r *Router) SendMsg(packetHeader *sharedstruct.SSPacketHeader, packetBody []byte) error {
	if r.busImpl != nil && packetHeader.DstBusID == r.busImpl.SelfBusId() {
		finish := beginRouterObserve("send", "local", packetHeader.Cmd)
		packet := &sharedstruct.SSPacket{
			Header: *packetHeader,
		}
		if len(packetBody) > 0 {
			packet.Body = make([]byte, len(packetBody))
			copy(packet.Body, packetBody)
		}
		if r.cbOnRecvSSPacket != nil {
			r.cbOnRecvSSPacket(packet)
		}
		finish(len(packetBody), nil)
		return nil
	}
	if r.busImpl == nil {
		return errors.New("router bus is not initialized")
	}

	finish := beginRouterObserve("send", "bus", packetHeader.Cmd)
	// 头编码到栈上数组：IBus.Send 会同步把 data1 拷贝进自己的帧缓冲，
	// 不会保留引用，因此这里无需堆分配。
	var headerBuf [86]byte
	_ = packetHeader.To(headerBuf[:])
	err := r.busImpl.Send(packetHeader.DstBusID, headerBuf[:], packetBody)
	finish(len(packetBody), err)
	if err != nil {
		e := fmt.Sprintf("failed to send bus message {header:%#v, bodyLen:%v} | %v",
			packetHeader, len(packetBody), err)
		logger.Errorf("%s", e)
		return errors.New(e)
	}
	return nil
}

func (r *Router) SendPbMsg(packetHeader *sharedstruct.SSPacketHeader, pbMsg proto.Message) error {
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
	return r.SendMsg(packetHeader, packetBody)
}

func (r *Router) SendMsgByBusId(busId uint32, routerKey, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	if busId == 0 {
		logger.Errorf("server instance is 0, fail to send {busId: %v, uid: %v, cmd: %X}", busId, uid, cmd)
		return errors.New("server instance is 0, fail to send")
	}

	packetHeader := sharedstruct.SSPacketHeader{
		SrcBusID:   r.SelfBusId(),
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

	return r.SendMsg(&packetHeader, data)
}

func (r *Router) SendPbMsgByBusId(busId uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	if logger.DebugEnabled() {
		logger.CmdDebugf(uint32(cmd),
			"SendPbMsgByBusId {dstBusId:%v, uid:%v, zone:%v, cmd:%v, bodyLen:%d, msgType:%s}",
			busId, uid, zone, uint32(cmd), len(data), protoMessageType(pbMsg))
	}
	return r.SendMsgByBusId(busId, 0, uid, zone, cmd, sendSeq, srcTransId, data)
}

func (r *Router) SendMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	dstBusId, routerKey := r.instanceMgr.GetSvrInsBySvrType(svrType, zone, uid, routerId)
	if dstBusId == 0 {
		logger.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %v}", svrType, uid, cmd)
		return errors.New("cannot get a server instance to send")
	}

	return r.SendMsgByBusId(dstBusId, routerKey, uid, zone, cmd, sendSeq, srcTransId, data)
}

func (r *Router) SendMsgByConn(uid, routerId uint64, zone, cmd uint32, srcTransId uint32, data []byte, ip, port uint32) error {
	svrType := misc.ServerTypeInCmd(cmd)
	dstBusId, routerKey := r.instanceMgr.GetSvrInsBySvrType(svrType, zone, uid, routerId)
	if dstBusId == 0 {
		logger.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %v}", svrType, uid, cmd)
		return errors.New("cannot get a server instance to send")
	}

	packetHeader := sharedstruct.SSPacketHeader{
		SrcBusID:   r.SelfBusId(),
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
		// 网关入口是调用链的根：为每个客户端请求生成 root trace，
		// 下游服务经 Transaction 透传，实现全链路日志关联。
		TraceID: sharedstruct.NewTraceID(),
		SpanID:  sharedstruct.NewSpanID(),
	}

	return r.SendMsg(&packetHeader, data)
}

// SendMsgBySvrTypeTraced 与 SendMsgBySvrType 相同，但把调用方的 trace 与
// deadline 透传到下游请求头（级联超时与全链路追踪）。
func (r *Router) SendMsgBySvrTypeTraced(tc sharedstruct.TraceContext, svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	dstBusId, routerKey := r.instanceMgr.GetSvrInsBySvrType(svrType, zone, uid, routerId)
	if dstBusId == 0 {
		logger.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %v}", svrType, uid, cmd)
		return errors.New("cannot get a server instance to send")
	}
	return r.SendMsgByBusIdTraced(tc, dstBusId, routerKey, uid, zone, cmd, sendSeq, srcTransId, data)
}

// SendMsgByBusIdTraced 与 SendMsgByBusId 相同，但透传 trace 与 deadline。
func (r *Router) SendMsgByBusIdTraced(tc sharedstruct.TraceContext, busId uint32, routerKey, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	if busId == 0 {
		logger.Errorf("server instance is 0, fail to send {busId: %v, uid: %v, cmd: %X}", busId, uid, cmd)
		return errors.New("server instance is 0, fail to send")
	}

	packetHeader := sharedstruct.SSPacketHeader{
		SrcBusID:   r.SelfBusId(),
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
	tc.ApplyTo(&packetHeader)

	return r.SendMsg(&packetHeader, data)
}

func (r *Router) BroadcastMsgByServerType(svrType uint32, uid uint64, cmd g1_protocol.CMD, sendSeq uint16, data []byte) error {
	instances := r.instanceMgr.GetAllSvrInsBySvrType(svrType)
	if len(instances) == 0 {
		return fmt.Errorf("cannot get a server instance to send {svrType: %v, uid: %v, cmd: %X}", svrType, uid, cmd)
	}

	var firstErr error
	for _, inst := range instances {
		if err := r.SendMsgByBusId(inst, 0, uid, 1, cmd, sendSeq, 0, data); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (r *Router) onRecvBusMsg(srcBusId uint32, data []byte) error {
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
	if r.cbOnRecvSSPacket != nil {
		r.cbOnRecvSSPacket(packet)
	}
	finish(len(packet.Body), nil)

	return nil
}

// ---------------------- package-level compatibility API ----------------------
// 以下包级函数委托给默认实例，保持既有调用方零改动。

func SelfBusId() uint32   { return defaultRouter.SelfBusId() }
func SelfSvrType() uint32 { return defaultRouter.SelfSvrType() }

// cb CbOnRecvSSPacket将由底层(bus)协程调用
func InitAndRun(selfBusId string, cb CbOnRecvSSPacket, busMQAddr string,
	routeRules map[uint32]uint32, registerAddr string) error {
	return defaultRouter.InitAndRun(selfBusId, cb, busMQAddr, routeRules, registerAddr)
}

func BeginShutdown() { defaultRouter.BeginShutdown() }

func Close() error { return defaultRouter.Close() }

func Snapshot() AdminSnapshot { return defaultRouter.Snapshot() }

// BusHealthy reports whether the underlying bus connection is currently up.
func BusHealthy() bool { return defaultRouter.BusHealthy() }

// ReadyCheck fails while the default router is shutting down or the bus is down.
func ReadyCheck() error { return defaultRouter.ReadyCheck() }

func SendMsg(packetHeader *sharedstruct.SSPacketHeader, packetBody []byte) error {
	return defaultRouter.SendMsg(packetHeader, packetBody)
}

func SendPbMsg(packetHeader *sharedstruct.SSPacketHeader, pbMsg proto.Message) error {
	return defaultRouter.SendPbMsg(packetHeader, pbMsg)
}

func SendMsgByBusId(busId uint32, routerKey, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	return defaultRouter.SendMsgByBusId(busId, routerKey, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendPbMsgByBusId(busId uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	return defaultRouter.SendPbMsgByBusId(busId, uid, zone, cmd, sendSeq, srcTransId, pbMsg)
}

func SendPbMsgByBusIdSimple(busId uint32, uid uint64, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return defaultRouter.SendPbMsgByBusId(busId, uid, 1, cmd, 0, 0, pbMsg)
}

func SendMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, data []byte) error {
	return defaultRouter.SendMsgBySvrType(svrType, routerId, uid, zone, cmd, sendSeq, srcTransId, data)
}

// SendPbMsgBySvrTypeTraced marshals pbMsg and sends it with the caller's
// trace context and deadline propagated in the packet header.
func SendPbMsgBySvrTypeTraced(tc sharedstruct.TraceContext, svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	return defaultRouter.SendMsgBySvrTypeTraced(tc, svrType, routerId, uid, zone, cmd, sendSeq, srcTransId, data)
}

// SendPbMsgByBusIdTraced marshals pbMsg and sends it to a specific bus id
// with the caller's trace context and deadline propagated.
func SendPbMsgByBusIdTraced(tc sharedstruct.TraceContext, busId uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	return defaultRouter.SendMsgByBusIdTraced(tc, busId, 0, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendMsgByConn(uid, routerId uint64, zone, cmd uint32, srcTransId uint32, data []byte, ip, port uint32) error {
	return defaultRouter.SendMsgByConn(uid, routerId, zone, cmd, srcTransId, data, ip, port)
}

func SendPbMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, sendSeq uint16, srcTransId uint32, pbMsg proto.Message) error {
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return err
	}
	return defaultRouter.SendMsgBySvrType(svrType, routerId, uid, zone, cmd, sendSeq, srcTransId, data)
}

func SendPbMsgBySvrTypeSimple(svrType uint32, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return SendPbMsgBySvrType(svrType, uid, uid, zone, cmd, 0, 0, pbMsg)
}

func SendPbMsgByRouter(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message) error {
	return SendPbMsgBySvrType(svrType, routerId, uid, zone, cmd, 0, 0, pbMsg)
}

func BroadcastMsgByServerType(svrType uint32, uid uint64, cmd g1_protocol.CMD, sendSeq uint16, data []byte) error {
	return defaultRouter.BroadcastMsgByServerType(svrType, uid, cmd, sendSeq, data)
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
	return defaultRouter.BroadcastMsgByServerType(svrType, uid, cmd, sendSeq, data)
}

func SendMsgBack(originalHeader sharedstruct.SSPacketHeader, srcTransId uint32, pbMsg proto.Message) {
	originalHeader.DstBusID = originalHeader.SrcBusID
	originalHeader.SrcBusID = SelfBusId()
	originalHeader.DstTransID = originalHeader.SrcTransID
	originalHeader.SrcTransID = srcTransId
	originalHeader.Cmd = originalHeader.Cmd + 1
	_ = SendPbMsg(&originalHeader, pbMsg)
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
	_ = SendPbMsg(&originalHeader, pbMsg)
}

// -------------------------------- private --------------------------------

func protoMessageType(msg proto.Message) string {
	if msg == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", msg)
}
