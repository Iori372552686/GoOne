package transaction

import (
	"context"
	"errors"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

// 哨兵错误（P1-02）：RegisterCmdE 在装配期校验并返回，使启动期错误以明确错误暴露，
// 而非 logger.Fatalf 杀进程或静默覆盖。
var (
	// ErrNilCmdHandler 在注册 nil handler 时返回。
	ErrNilCmdHandler = errors.New("transaction: nil cmd handler")
	// ErrDuplicateCmd 在同一 cmd 已注册时返回（不再 last-write-wins）。
	ErrDuplicateCmd = errors.New("transaction: duplicate cmd")
	// ErrRegisterAfterStart 在 InitAndRun 之后注册时返回。
	ErrRegisterAfterStart = errors.New("transaction: register after start")
)

type ITransactionMgr interface {
	InitAndRun(maxTrans int32, useUidLock bool, maxUidPendingPacket int)
	InitAndRunWithConfig(cfg TransactionMgrConfig)

	// RegisterCmd 是兼容入口：内部委托 RegisterCmdE 并仅记录错误，不 Fatal、不覆盖
	//（P1-02）。生产装配应优先使用 RegisterCmdE。
	RegisterCmd(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc)
	// RegisterCmdE 注册一个 cmd handler，返回明确哨兵错误（P1-02）：
	//   - nil handler → ErrNilCmdHandler
	//   - 重复 cmd → ErrDuplicateCmd
	//   - InitAndRun 之后注册 → ErrRegisterAfterStart
	RegisterCmdE(cmd g1_protocol.CMD, cmdHandler cmd_handler.CmdHandlerFunc) error
	ProcessSSPacket(packet *sharedstruct.SSPacket)
	// SendPbMsgToMyself 把消息直接投入本进程的事务队列（不经网络），
	// 用于需要与业务 handler 按串行键（rid/uid）串行执行的内部任务。
	SendPbMsgToMyself(selfBusId uint32, rid uint64, uid uint64, zone uint32, cmd g1_protocol.CMD, pbMsg proto.Message)
	StatsSnapshot() TransactionMgrStats
	Close(ctx context.Context) error
}
