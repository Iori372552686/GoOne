package web_gin

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	handlers        = make(map[string]ICmdWebHandler)
	handlersRWMutex sync.RWMutex
)

/**
* @Description: reg
* @param: key
* @param: value
* @Author: Iori
* @Date: 2022-04-26 17:31:33
**/
func Register(key string, value ICmdWebHandler) {
	handlersRWMutex.Lock()
	defer handlersRWMutex.Unlock()
	handlers[key] = value

	return
}

/**
* @Description:  get handlers
* @param: key
* @return: value
* @return: ok
* @Author: Iori
* @Date: 2022-04-26 17:31:21
**/
func GetHandlers(key string) (value ICmdWebHandler, ok bool) {
	handlersRWMutex.RLock()
	defer handlersRWMutex.RUnlock()

	value, ok = handlers[key]
	return
}

/*  ICmdWsHandler
*  @Description:
 */
type ICmdWebHandler interface {
	ProcessCmd(cParams, data *map[string]interface{}) *gin.H
}
