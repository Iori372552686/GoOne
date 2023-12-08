package ws_server

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"sync"
)

var (
	handlers        = make(map[string]ICmdWsHandler)
	handlersRWMutex sync.RWMutex
)

func Register(key string, value ICmdWsHandler) {
	handlersRWMutex.Lock()
	defer handlersRWMutex.Unlock()
	handlers[key] = value

	return
}

func GetHandlers(key string) (value ICmdWsHandler, ok bool) {
	handlersRWMutex.RLock()
	defer handlersRWMutex.RUnlock()

	value, ok = handlers[key]

	return
}

// proc data
func (c *Client) ProcessData(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("%s 处理数据 error : %v", c.Addr, r)
		}
	}()

	clientManager.HandlerFunc(c, message)
	return
}

type ICmdWsHandler interface {
	ProcessCmd(client *Client, data []byte) int
}
