package globals

import (
	`GoOne/lib/transaction`
	`GoOne/src/infosvr/info`
)

var TransMgr = transaction.NewTransactionMgr()
var InfoMgr = info.NewInfoMgr()
