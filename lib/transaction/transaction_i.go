package transaction

import (
	`GoOne/lib/cmd_handler`
	`GoOne/lib/sharedstruct`
)

type ITransactionMgr interface {
	// parameters:
	//   useUidLock:
	//     true: 每个uid最多只会有一个在执行中的协程（一般用于内存中留有uid相关信息的svr，如mainsvr）（后面的消息进队列）
	//     false:协程数与uid无关（一般用于无状态类的svr，如dbsvr）
	//   maxUidPendingPacket:
	//     当useUidLock=true时，此值为每个uid的消息等待队列的长度。
	InitAndRun(maxTrans int32, useUidLock bool, maxUidPendingPacket int)

	RegisterCmd(cmd uint32, cmdHandler cmd_handler.ICmdHandler)
	ProcessSSPacket(packet *sharedstruct.SSPacket)
}