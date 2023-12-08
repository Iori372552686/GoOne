package globals

import (
	"github.com/Iori372552686/GoOne/lib/db/mysql"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

var TransMgr = transaction.NewTransactionMgr()
var MysqlMgr = mysql.NewMysqlMgr()
