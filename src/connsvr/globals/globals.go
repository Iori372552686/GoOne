package globals

import (
	"GoOne/lib/service/transaction"
	"GoOne/src/connsvr/tcp_server"
)

var TransMgr = transaction.NewTransactionMgr()
var ConnTcpSvr = tcp_server.NewTcpSvr()
