package globals

import (
	"GoOne/lib/transaction"
	"GoOne/src/connsvr/tcp_server"
)

var TransMgr = transaction.NewTransactionMgr()
var ConnTcpSvr = tcp_server.NewTcpSvr()
