package ssrpc

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

type sendBacker interface {
	SendMsgBack(pbMsg proto.Message)
}

// SendMsgBackWithCmd tries to send response with a specified cmd.
//
// - If underlying context supports SendMsgBackWithCmd, it will be used.
// - Otherwise it falls back to SendMsgBack (cmd+1 convention).
func SendMsgBackWithCmd(ctx sendBacker, cmd g1_protocol.CMD, pbMsg proto.Message) {
	if ctx == nil || pbMsg == nil {
		return
	}
	if v, ok := any(ctx).(interface {
		SendMsgBackWithCmd(cmd g1_protocol.CMD, pbMsg proto.Message)
	}); ok {
		v.SendMsgBackWithCmd(cmd, pbMsg)
		return
	}
	ctx.SendMsgBack(pbMsg)
}


