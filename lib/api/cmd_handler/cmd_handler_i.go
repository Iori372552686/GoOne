package cmd_handler

import (
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
)

type CmdHandlerFunc func(c IContext, data []byte) g1_protocol.ErrorCode

// Transaction 实现了这个借口，在事务运行时保存了上下文
type IContext interface {
	Uid() uint64
	Zone() uint32
	Rid() uint64
	OriSrcBusId() uint32
	Ip() uint32
	Flag() uint32

	ParseMsg(data []byte, msg proto.Message) error

	CallMsgBySvrType(svrType uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error                                         // 常规rpc call
	CallMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error                         // 带自定义路由的 call
	CallOtherMsgBySvrType(svrType uint32, routerId, uid uint64, zone uint32, cmd g1_protocol.CMD, req proto.Message, rsp proto.Message) error //附带其他玩家id的call
	SendMsgBack(pbMsg proto.Message)                                                                                                          //rpc msg back
	SendMsgByServerType(svrType uint32, cmd g1_protocol.CMD, req proto.Message) error                                                         // 常规rpc send
	SendMsgByRouter(svrType uint32, routerId uint64, cmd g1_protocol.CMD, req proto.Message) error                                            //带自定义路由的 send
	//SendPbMsgByBusId(busId uint32, cmd uint32, req proto.Message) error

	Errorf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}
