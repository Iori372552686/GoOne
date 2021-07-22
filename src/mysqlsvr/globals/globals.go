package globals

import (
	`GoOne/lib/mysql`
	`GoOne/lib/transaction`
)

var TransMgr = transaction.NewTransactionMgr()
var MysqlMgr = mysql.NewMysqlMgr()

