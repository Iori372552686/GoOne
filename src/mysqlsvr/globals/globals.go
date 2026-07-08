package globals

import (
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

var TransMgr = transaction.NewTransactionMgr()
var OrmMgr = orm.NewOrmMgr()
