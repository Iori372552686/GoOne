package transaction

import (
	"context"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type ITransactionMgr interface {
	InitAndRun(maxTrans int32, useUidLock bool, maxUidPendingPacket int)
	InitAndRunWithConfig(cfg TransactionMgrConfig)

	RegisterCmd(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc)
	ProcessSSPacket(packet *sharedstruct.SSPacket)
	// SendPbMsgToMyself 把消息直接投入本进程的事务队列（不经网络），
	// 用于需要与业务 handler 按串行键（rid/uid）串行执行的内部任务。
	SendPbMsgToMyself(selfBusId uint32, rid uint64, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message)
	StatsSnapshot() TransactionMgrStats
	Close(ctx context.Context) error
}
