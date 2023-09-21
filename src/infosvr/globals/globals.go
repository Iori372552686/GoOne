package globals

import (
	"GoOne/lib/service/transaction"
	"GoOne/src/infosvr/info"
)

var TransMgr = transaction.NewTransactionMgr()
var InfoMgr = info.NewInfoMgr()
