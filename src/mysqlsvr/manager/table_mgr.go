package manager

import (
	"github.com/Iori372552686/GoOne/lib/service/async"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

const (
	ASYNC_COUNT = 15
)

var (
	tables   = []interface{}{}
	handlers = async.NewAsyncPool(ASYNC_COUNT)
)

type IUpdate interface {
	GetUpdateTime() int64
}

func GetTables() []interface{} {
	return tables
}

func init() {
	// 启动协程
	for _, handler := range handlers {
		handler.Start()
	}

	// 注册mysql表
	tables = append(tables,
		new(g1_protocol.MysqlTexasRoomInfo),
		new(g1_protocol.MysqlTexasPlayerInfo),
		new(g1_protocol.MysqlTexasGameInfo),
	)
}

func Push(id int64, f func()) {
	handlers[id%ASYNC_COUNT].Push(f)
}

func Close() {
	for _, handler := range handlers {
		handler.Stop()
	}
}
