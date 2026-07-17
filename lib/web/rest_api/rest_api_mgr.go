package rest_api

import (
	"github.com/Iori372552686/GoOne/lib/api/http_sign"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/gfunc"
	"sync"
)

/**
 * RestApiMgr
 * @Description:
**/
type RestApiMgr struct {
	mu        sync.RWMutex
	Instances map[string]*RestApi

	//private
	lastTick int64
}

/**
* @Description: 创建restApi管理器
* @return: *RestApiMgr
* @Author: Iori
* @Date: 2022-02-14 11:28:59
**/
func NewRestApiMgr() *RestApiMgr {
	r := &RestApiMgr{}
	r.Instances = make(map[string]*RestApi)

	return r
}

/**
* @Description: 设置restApi实例
* @param: key
* @param: impl
* @Author: Iori
* @Date: 2022-02-14 16:13:53
**/
func (self *RestApiMgr) SetRestIns(key string, impl *RestApi) {
	if key == "" || impl == nil {
		return
	}

	self.mu.Lock()
	defer self.mu.Unlock()
	self.Instances[key] = impl
}

/**
* @Description: 获取restApi实例
* @receiver: self
* @param: key
* @param: o
* @Author: Iori
* @Date: 2022-02-14 11:29:20
**/
func (self *RestApiMgr) GetRestIns(keys ...string) *RestApi {
	self.mu.RLock()
	defer self.mu.RUnlock()
	if len(keys) == 0 {
		return self.Instances["default"]
	} else {
		return self.Instances[keys[0]]
	}
}

/**
* @Description: 计数
* @receiver: self
* @return: int
* @Author: Iori
* @Date: 2022-02-14 11:29:45
**/
func (self *RestApiMgr) Count() int {
	if self == nil {
		return 0
	}
	self.mu.RLock()
	defer self.mu.RUnlock()
	return len(self.Instances)
}

/**
* @Description: 初始化RestApi管理器
* @receiver: self
* @param: cfgs
* @return: error
* @Author: Iori
* @Date: 2022-02-14 11:29:45
**/
func (self *RestApiMgr) Init(cfgs []Config, signs *http_sign.SignMgr) {
	logger.Infof("RestApiMgr   InsInit.. ")

	for _, conf := range cfgs {
		self.SetRestIns(conf.ServiceName, NewRestInstances(conf, signs))
	}

	logger.Infof("RestApiMgr   InsInit... Done !")
}

/**
* @Description: tick
* @receiver: self
* @param: nowMs
* @Author: Iori
* @Date: 2022-02-14 11:39:54
**/
func (self *RestApiMgr) Tick(nowMs int64) {
	defer gfunc.CheckRecover()
	return
}
