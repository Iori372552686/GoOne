package globals

import (
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/util/idgen"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
)

var TransMgr = transaction.NewTransactionMgr()
var RoleMgr = role.NewRoleMgr()
var IDGen *idgen.TIDGen
