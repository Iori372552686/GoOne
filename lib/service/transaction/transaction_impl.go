package transaction

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type Transaction struct {
	OriPacketHeader sharedstruct.SSPacketHeader
	// CurFrameHeader sharedstruct.SSPacketHeader

	transID  uint32
	sendSeq  uint16
	chanIn   chan *sharedstruct.SSPacket
	traceCtx context.Context
}

func newTransaction(transID uint32, oriPacketHeader sharedstruct.SSPacketHeader,
	chanIn chan *sharedstruct.SSPacket) *Transaction {
	t := new(Transaction)
	t.transID = transID
	t.OriPacketHeader = oriPacketHeader
	t.chanIn = chanIn
	t.sendSeq = 0
	t.traceCtx = contextForSSPacketTrace(oriPacketHeader.SrcBusID, oriPacketHeader.SrcTransID, oriPacketHeader.CmdSeq, oriPacketHeader.Cmd)
	return t
}

func (t *Transaction) Context() context.Context {
	if t == nil || t.traceCtx == nil {
		return context.Background()
	}
	return t.traceCtx
}

func (t *Transaction) Errorf(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v|%v] %v", t.Uid(), t.Rid(), t.TransID(), format)
	logger.ErrorDepth(1, fmt.Sprintf(f, args...))
}

func (t *Transaction) Warningf(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v|%v] %v", t.Uid(), t.Rid(), t.TransID(), format)
	logger.WarningDepth(1, fmt.Sprintf(f, args...))
}

func (t *Transaction) Infof(format string, args ...interface{}) {
	f := fmt.Sprintf("[%v|%v|%v] %v", t.Uid(), t.Rid(), t.TransID(), format)
	logger.InfoDepth(1, fmt.Sprintf(f, args...))
}

func (t *Transaction) Debugf(format string, args ...interface{}) {
	t.DebugDepthf(1, format, args...)
}

func (t *Transaction) DebugDepthf(depth int, format string, args ...interface{}) {
	if !logger.DebugEnabled() {
		return
	}
	f := fmt.Sprintf("[%v|%v|%v] %v", t.Uid(), t.Rid(), t.TransID(), format)
	logger.CmdDebugDepthf(t.Cmd(), 1+depth, f, args...)
}

func (t *Transaction) run(cmdHandler cmd_handler.CmdHandlerFunc, packet *sharedstruct.SSPacket, chanRet chan<- uint32) {
	start := time.Now()
	ret := g1_protocol.ErrorCode_ERR_OK
	safego.SafeFunc(func() {
		ret = cmdHandler(t, packet.Body)
		if ret != g1_protocol.ErrorCode_ERR_OK {
			logger.Errorf("cmdHandler failed: %v", ret)
		}
	})
	observeTransactionHandler(t.Cmd(), ret, time.Since(start))

	chanRet <- t.transID
}

func (t *Transaction) Uid() uint64 {
	return t.OriPacketHeader.Uid
}

func (t *Transaction) Zone() uint32 {
	return t.OriPacketHeader.Zone
}

func (t *Transaction) Rid() uint64 {
	return t.OriPacketHeader.RouterID
}

func (t *Transaction) Cmd() uint32 {
	return t.OriPacketHeader.Cmd
}

func (t *Transaction) OriSrcBusId() uint32 {
	return t.OriPacketHeader.SrcBusID
}

func (t *Transaction) TransID() uint32 {
	return t.transID
}

func (t *Transaction) Ip() uint32 {
	return t.OriPacketHeader.Ip
}

func (t *Transaction) Flag() uint32 {
	return t.OriPacketHeader.Flag
}

func (t *Transaction) ParseMsg(data []byte, msg proto.Message) error {
	err := proto.Unmarshal(data, msg)
	if err != nil {
		t.Warningf("Fail to unmarshal req | %v", err)
		return err
	}
	t.Debugf("parse msg {bodyLen:%d, msgType:%s}", len(data), protoMessageType(msg))
	return nil
}

func (t *Transaction) SendMsgBack(pbMsg proto.Message) {
	router.SendMsgBack(t.OriPacketHeader, t.transID, pbMsg)
}

// SendMsgBackWithCmd sends a response to the original caller but overrides cmd.
// This is primarily used by IDL-driven ssrpc wrappers when cmd_resp is explicitly specified.
func (t *Transaction) SendMsgBackWithCmd(cmd g1_protocol.CMD, pbMsg proto.Message) {
	router.SendMsgBackWithCmd(t.OriPacketHeader, t.transID, cmd, pbMsg)
}

func (t *Transaction) CallMsgBySvrType(svrType uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	return t.CallOtherMsgBySvrType(svrType, t.Uid(), t.Uid(), t.Zone(), cmd, req, rsp)
}

func (t *Transaction) CallMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	return t.CallOtherMsgBySvrType(svrType, routerId, t.Uid(), t.Zone(), cmd, req, rsp)
}

func (t *Transaction) CallOtherMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	t.Debugf("CallMsgBySvrType {dstSvrType:%v, routerId:%v, uid:%v, zone:%v, cmd:%v, reqType:%s}",
		svrType, routerId, uid, zone, uint32(cmd), protoMessageType(req))
	t.sendSeq += 1
	err := router.SendPbMsgBySvrType(svrType, routerId, uid, zone, cmd, t.sendSeq, t.TransID(), req)
	if err != nil {
		logger.Error(err)
		return err
	}

	return t.waitRsp(svrType, 0, cmd, time.Second*3, req, rsp)
}

func (t *Transaction) SendMsgByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("SendMsgByServerType {dstSvrType:%v, cmd:%v, reqType:%s}", svrType, uint32(cmd), protoMessageType(req))
	t.sendSeq += 1
	err := router.SendPbMsgBySvrTypeSimple(svrType, t.Uid(), t.Zone(), cmd, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) SendMsgByRouter(svrType uint32, rid uint64, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("SendMsgByRouter {dstSvrType:%v, rid:%v, cmd:%v, reqType:%s}", svrType, rid, uint32(cmd), protoMessageType(req))
	t.sendSeq += 1
	err := router.SendPbMsgByRouter(svrType, rid, t.Uid(), t.Zone(), cmd, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) BroadcastByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("BroadcastByServerType {dstSvrType:%v, cmd:%v, reqType:%s}", svrType, uint32(cmd), protoMessageType(req))
	t.sendSeq += 1
	err := router.BroadcastPbMsgByServerType(svrType, t.Uid(), cmd, t.sendSeq, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) CallMsgByBusId(busId uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	t.Debugf("CallMsgByBusId {dstBusId:%v, cmd:%v, reqType:%s}", busId, uint32(cmd), protoMessageType(req))
	t.sendSeq += 1
	err := router.SendPbMsgByBusId(busId, t.Uid(), t.Zone(), cmd, t.sendSeq, t.TransID(), req)
	if err != nil {
		logger.Error(err)
		return err
	}

	return t.waitRsp(0, busId, cmd, time.Second*3, req, rsp)
}

func contextForSSPacketTrace(srcBusID uint32, srcTransID uint32, cmdSeq uint16, cmd uint32) context.Context {
	if srcBusID == 0 || srcTransID == 0 {
		return context.Background()
	}
	seed := make([]byte, 14)
	binary.BigEndian.PutUint32(seed[0:4], srcBusID)
	binary.BigEndian.PutUint32(seed[4:8], srcTransID)
	binary.BigEndian.PutUint16(seed[8:10], cmdSeq)
	binary.BigEndian.PutUint32(seed[10:14], cmd)
	traceHash := sha256.Sum256(seed)
	spanSeed := append([]byte("span:"), seed...)
	spanHash := sha256.Sum256(spanSeed)

	ctx := context.Background()
	ctx = context.WithValue(ctx, "goone.ssrpc.trace_id", hex.EncodeToString(traceHash[:16]))
	ctx = context.WithValue(ctx, "goone.ssrpc.span_id", hex.EncodeToString(spanHash[:8]))
	return ctx
}

func (t *Transaction) waitRsp(dstSvrType uint32, dstSvrIns uint32, cmd g1_protocol.CMD,
	d time.Duration, req proto.Message, rsp proto.Message) error {
	ti := time.NewTimer(d)
	defer ti.Stop()
	for {
		select {
		case <-ti.C:
			observeTransactionTimeout("wait_rsp", cmd)
			logger.Errorf("timeout to CallMsgBySvrType {svrType:%v, svrIns:%v, uid:%v, cmd:%v, reqType:%s}",
				dstSvrType, dstSvrIns, t.Uid(), cmd, protoMessageType(req))
			return errors.New("timeout")
		case packet, ok := <-t.chanIn:
			if !ok {
				logger.Errorf("Failed to CallMsgBySvrType as chanInPacket is closed "+
					"{svrType:%v, svrIns:%v, uid:%v, cmd:%v, rid:%v, reqType:%s}",
					dstSvrType, dstSvrIns, t.Uid(), cmd, t.Rid(), protoMessageType(req))
				return errors.New("channel is closed")
			}
			// Primary match is CmdSeq (Transaction-driven request/response correlation).
			// Historically we also enforced rspCmd == reqCmd+1, but IDL-driven ssrpc may override cmd_resp.
			if packet.Header.CmdSeq != t.sendSeq {
				logger.Warningf("Received a packet which is not what I'm waiting for "+
					"{dstSvrType:%v, dstSvrIns:%v, uid:%v, cmd:%v, rid:%v, reqType:%s, recvPacket:%#v}",
					dstSvrType, dstSvrIns, t.Uid(), cmd, t.Rid(), protoMessageType(req), packet.Header)
				break
			}

			if packet.Header.Cmd != uint32(cmd)+1 {
				logger.Warningf("Received a rsp with unexpected cmd (still decoding by CmdSeq) "+
					"{expectRspCmd:%v, gotRspCmd:%v, uid:%v, rid:%v, reqCmd:%v}",
					uint32(cmd)+1, packet.Header.Cmd, t.Uid(), t.Rid(), uint32(cmd))
			}

			err := proto.Unmarshal(packet.Body, rsp)
			t.Debugf("Received rsp {rspCmd:%v, bodyLen:%d, rspType:%s}", packet.Header.Cmd, len(packet.Body), protoMessageType(rsp))
			return err
		}
		// Mismatched packet: restart the timeout window, reusing the timer.
		if !ti.Stop() {
			select {
			case <-ti.C:
			default:
			}
		}
		ti.Reset(d)
	}
}

func protoMessageType(msg proto.Message) string {
	if msg == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", msg)
}
