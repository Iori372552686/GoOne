package tcp_server

import (
	"net"
	"sync"
)

var (
	handlers        = make(map[string]ICmdTcpHandler)
	handlersRWMutex sync.RWMutex
)

func Register(key string, value ICmdTcpHandler) {
	handlersRWMutex.Lock()
	defer handlersRWMutex.Unlock()
	handlers[key] = value

	return
}

func GetHandlers(key string) (value ICmdTcpHandler, ok bool) {
	handlersRWMutex.RLock()
	defer handlersRWMutex.RUnlock()

	value, ok = handlers[key]

	return
}

type ICmdTcpHandler interface {
	ProcessCmd(conn net.Conn, data []byte) int
}
