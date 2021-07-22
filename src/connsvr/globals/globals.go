package globals

import (
	`GoOne/lib/transaction`
	web `GoOne/lib/web/client`
)

var TransMgr = transaction.NewTransactionMgr()
var ClientMgr = web.NewClientMgr()
