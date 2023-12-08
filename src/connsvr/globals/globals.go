package globals

import (
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/src/connsvr/tcp_server"
)

var TransMgr = transaction.NewTransactionMgr()
var ConnTcpSvr = tcp_server.NewTcpSvr()
