package globals

import (
	"github.com/Iori372552686/GoOne/lib/db/mysql"
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

var TransMgr = transaction.NewTransactionMgr()
var MysqlMgr = mysql.NewMysqlMgr()
var OrmMgr = orm.NewOrmMgr()
