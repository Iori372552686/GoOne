
package web

import (
	"fmt"
	"sync"
)

type DisposeFunc func(client *Client, seq string, message []byte) (code uint32, msg string, data interface{})

var (
	handlers        = make(map[string]DisposeFunc)
	handlersRWMutex sync.RWMutex
)

// 注册
func Register(key string, value DisposeFunc) {
	handlersRWMutex.Lock()
	defer handlersRWMutex.Unlock()
	handlers[key] = value

	return
}

func getHandlers(key string) (value DisposeFunc, ok bool) {
	handlersRWMutex.RLock()
	defer handlersRWMutex.RUnlock()

	value, ok = handlers[key]

	return
}

// 处理数据
func ProcessData(client *Client, message []byte) {

	fmt.Println("处理数据", client.Addr, string(message))

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("处理数据 stop", r)
		}
	}()


	//globals.TransMgr.ProcessSSPacket(nil)
	//client.SendMsg(headByte)
	fmt.Println("acc_response send", client.Addr, client.AppId, client.UserId)

	return
}
