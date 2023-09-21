package cmd_handler

import (
	"github.com/golang/protobuf/proto"
)

// Transaction 实现了这个借口，在事务运行时保存了上下文
type IContext interface {
	Uid() uint64
	OriSrcBusId() uint32
	Ip() uint32
	Flag() uint32

	ParseMsg(data []byte, msg proto.Message) error

	CallMsgBySvrType(svrType uint32, cmd uint32, req proto.Message, rsp proto.Message) error
	CallOtherMsgBySvrType(svrType uint32, cmd uint32, uid uint64, req proto.Message, rsp proto.Message) error
	SendMsgBack(pbMsg proto.Message)
	SendMsgByServerType(svrType uint32, cmd uint32, req proto.Message) error
	// SendPbMsgByBusId(busId uint32, cmd uint32, req proto.Message) error


	Errorf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

type ICmdHandler interface {
	ProcessCmd(context IContext, data []byte) int
}