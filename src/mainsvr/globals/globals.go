package globals

import (
	"GoOne/lib/service/transaction"
	"GoOne/src/mainsvr/role"
)

var TransMgr = transaction.NewTransactionMgr()
var RoleMgr = role.NewRoleMgr()
