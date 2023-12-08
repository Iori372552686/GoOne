package globals

import (
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/src/infosvr/info"
)

var TransMgr = transaction.NewTransactionMgr()
var InfoMgr = info.NewInfoMgr()
