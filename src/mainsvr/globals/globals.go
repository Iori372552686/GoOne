package globals

import (
	`GoOne/lib/transaction`
	`GoOne/src/mainsvr/role`
)

var TransMgr = transaction.NewTransactionMgr()
var RoleMgr = role.NewRoleMgr()
