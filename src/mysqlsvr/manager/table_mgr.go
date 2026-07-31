package manager

import (
	"github.com/Iori372552686/GoOne/lib/service/async"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

const (
	ASYNC_COUNT = 15
)

// tables 是 ORM 需要同步的表清单。包级变量初始化时填充，无需 init（// 移除 package init 中的表注册，改为声明式）。
var tables = []interface{}{
	new(g1_protocol.MysqlTexasRoomInfo),
	new(g1_protocol.MysqlTexasPlayerInfo),
	new(g1_protocol.MysqlTexasGameInfo),
}

// handlers 是异步写库 worker 池。NewAsyncPool 仅创建对象（status=STOP），不启动
// goroutine；必须由 Start 显式启动（移除 package init 中的 worker 启动）。
var handlers = async.NewAsyncPool(ASYNC_COUNT)

type IUpdate interface {
	GetUpdateTime() int64
}

func GetTables() []interface{} {
	return tables
}

// Start 启动异步写库 worker 池。必须在接受任何 Push 之前调用；未启动时 Push 会
// 静默丢弃任务（async.Async 的 status 检查）。
func Start() {
	for _, handler := range handlers {
		handler.Start()
	}
}

func Push(id int64, f func()) {
	handlers[id%ASYNC_COUNT].Push(f)
}

func Close() {
	for _, handler := range handlers {
		handler.Stop()
	}
}
