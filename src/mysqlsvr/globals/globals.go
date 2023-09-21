package globals

import (
	"GoOne/lib/db/mysql"
	"GoOne/lib/service/transaction"
)

var TransMgr = transaction.NewTransactionMgr()
var MysqlMgr = mysql.NewMysqlMgr()
