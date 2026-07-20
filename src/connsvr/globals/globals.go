package globals

import (
	"github.com/Iori372552686/GoOne/lib/net/net_mgr"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
)

var (
	TransMgr               = transaction.NewTransactionMgr()
	ConnTcpSvr             = net_mgr.NewTcpSvr()
	ConnWsSvr              = net_mgr.NewWsTcpSvr()
	ConnKcpSvr             = net_mgr.NewKcpSvr()
	SignMgr                = http_sign.NewSignMgr()
	RestMgr                = rest_api.NewRestApiMgr()
	ClientPacketDispatcher = ssrpc.NewDispatcher()
)
