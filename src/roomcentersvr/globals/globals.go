package globals

import (
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr"
)

var TransMgr = transaction.NewTransactionMgr()
var RoomListMgr = room_mgr.NewRoomMgr()
