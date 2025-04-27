package transaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/util/safego"
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
)

type iTransaction interface {
	cmd_handler.IContext
	run(transID uint32, trans interface{}, packet *sharedstruct.SSPacket,
		chanIn <-chan *sharedstruct.SSPacket, chanTransRet chan<- transRet)
}

type Transaction struct {
	OriPacketHeader sharedstruct.SSPacketHeader
	// CurFrameHeader sharedstruct.SSPacketHeader

	transID uint32
	sendSeq uint16
	chanIn  chan *sharedstruct.SSPacket
}

func newTransaction(transID uint32, oriPacketHeader sharedstruct.SSPacketHeader,
	chanIn chan *sharedstruct.SSPacket) *Transaction {
	t := new(Transaction)
	t.transID = transID
	t.OriPacketHeader = oriPacketHeader
	t.chanIn = chanIn
	t.sendSeq = 0
	return t
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
	f := fmt.Sprintf("[%v|%v|%v] %v", t.Uid(), t.Rid(), t.TransID(), format)
	logger.CmdDebugDepthf(t.Cmd(), 1+depth, f, args...)
}

func (t *Transaction) run(cmdHandler cmd_handler.CmdHandlerFunc, packet *sharedstruct.SSPacket, chanRet chan<- uint32) {
	safego.SafeFunc(func() {
		ret := cmdHandler(t, packet.Body)
		if ret != g1_protocol.ErrorCode_ERR_OK {
			logger.Errorf("cmdHandler failed: %v", ret)
		}
	})

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
	t.Debugf("parse msg: %#v", msg.String())
	return nil
}

func (t *Transaction) SendMsgBack(pbMsg proto.Message) {
	router.SendMsgBack(t.OriPacketHeader, t.transID, pbMsg)
}

func (t *Transaction) CallMsgBySvrType(svrType uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	return t.CallOtherMsgBySvrType(svrType, t.Uid(), t.Uid(), t.Zone(), cmd, req, rsp)
}

func (t *Transaction) CallMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	return t.CallOtherMsgBySvrType(svrType, routerId, t.Uid(), t.Zone(), cmd, req, rsp)
}

func (t *Transaction) CallOtherMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	t.Debugf("CallMsgBySvrType: %#v", req.String())
	t.sendSeq += 1
	err := router.SendPbMsgBySvrType(svrType, routerId, uid, zone, cmd, t.sendSeq, t.TransID(), req)
	if err != nil {
		logger.Error(err)
		return err
	}

	return t.waitRsp(svrType, 0, cmd, time.Second*3, req, rsp)
}

func (t *Transaction) SendMsgByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("SendMsgByServerType: %#v", req.String())
	t.sendSeq += 1
	err := router.SendPbMsgBySvrTypeSimple(svrType, t.Uid(), t.Zone(), cmd, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) SendMsgByRouter(svrType uint32, rid uint64, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("SendMsgByRouter: %#v", req.String())
	t.sendSeq += 1
	err := router.SendPbMsgByRouter(svrType, rid, t.Uid(), t.Zone(), cmd, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) BroadcastByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error {
	t.Debugf("BroadcastByServerType: %#v", req.String())
	t.sendSeq += 1
	err := router.BroadcastPbMsgByServerType(svrType, t.Uid(), cmd, t.sendSeq, req)
	if err != nil {
		logger.Error(err)
	}
	return err
}

func (t *Transaction) CallMsgByBusId(busId uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error {
	t.Debugf("CallMsgByBusId: %#v", req.String())
	t.sendSeq += 1
	err := router.SendPbMsgByBusId(busId, t.Uid(), t.Zone(), cmd, t.sendSeq, t.TransID(), req)
	if err != nil {
		logger.Error(err)
		return err
	}

	return t.waitRsp(0, busId, cmd, time.Second*3, req, rsp)
}

func (t *Transaction) waitRsp(dstSvrType uint32, dstSvrIns uint32, cmd g1_protocol.CMD,
	d time.Duration, req proto.Message, rsp proto.Message) error {
	ti := time.NewTimer(d)
	defer ti.Stop()
	for {
		select {
		case <-ti.C:
			logger.Errorf("timeout to CallMsgBySvrType {svrType:%v, svrIns:%v, uid:%v, cmd:%v, req:%#v}",
				dstSvrType, dstSvrIns, t.Uid(), cmd, req.String())
			return errors.New("timeout")
		case packet, ok := <-t.chanIn:
			if !ok {
				logger.Errorf("Failed to CallMsgBySvrType as chanInPacket is closed "+
					"{svrType:%v, svrIns:%v, uid:%v, cmd:%v, rid:%v req:%#v}",
					dstSvrType, dstSvrIns, t.Uid(), cmd, t.Rid(), req.String())
				return errors.New("channel is closed")
			}
			if packet.Header.CmdSeq != t.sendSeq || packet.Header.Cmd != uint32(cmd)+1 {
				logger.Warningf("Received a packet which is not what I'm waiting for "+
					"{dstSvrType:%v, dstSvrIns:%v, uid:%v, cmd:%v, rid:%v,req:%#v, recvPacket:%#v}",
					dstSvrType, dstSvrIns, t.Uid(), cmd, t.Rid(), req.String(), packet.Header)
			} else {
				err := proto.Unmarshal(packet.Body, rsp)
				t.Debugf("Received a rsp: %#v", rsp.String())
				return err
			}
		}
		ti.Stop()
		ti = time.NewTimer(d)
	}
}
