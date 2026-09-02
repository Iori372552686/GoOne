package globals

import (
	gormdb "github.com/Iori372552686/GoOne/lib/db/gorm"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

var TransMgr = transaction.NewTransactionMgr()
var DBMgr = gormdb.NewManager()
