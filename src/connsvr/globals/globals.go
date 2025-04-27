package globals

import (
	"github.com/Iori372552686/GoOne/lib/api/http_sign"
	"github.com/Iori372552686/GoOne/lib/api/rest_api"
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

var (
	TransMgr   = transaction.NewTransactionMgr()
	ConnTcpSvr = net_mgr.NewTcpSvr()
	ConnWsSvr  = net_mgr.NewWsTcpSvr()
	SignMgr    = http_sign.NewSignMgr()
	RestMgr    = rest_api.NewRestApiMgr()
)
