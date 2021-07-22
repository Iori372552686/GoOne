package globals

import (
	`bian/src/bian_newFrame/lib/transaction`
	`bian/src/bian_newFrame/src/infosvr/info`
)

var TransMgr = transaction.NewTransactionMgr()
var InfoMgr = info.NewInfoMgr()
