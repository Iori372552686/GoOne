// by  Iori  2022/2/14
package http_sign

import (
	"github.com/Iori372552686/GoOne/common/gfunc"
	"github.com/Iori372552686/GoOne/lib/api/logger"
)

/*
*  SignMgr
*  @Description:
 */
type SignMgr struct {
	Instances map[string]*HttpSign

	//private
	lastTick int64
}

/**
* @Description: 创建签名管理器
* @return: *SignMgr
* @Author: Iori
* @Date: 2022-02-14 11:28:59
**/
func NewSignMgr() *SignMgr {
	r := &SignMgr{}
	r.Instances = make(map[string]*HttpSign)

	return r
}

/**
* @Description: 设置签名类型实例
* @param: key
* @param: impl
* @Author: Iori
* @Date: 2022-02-14 16:13:53
**/
func (self *SignMgr) SetSignIns(key string, impl *HttpSign) {
	self.Instances[key] = impl
}

/**
* @Description: 获取签名类型
* @receiver: self
* @param: key
* @param: o
* @Author: Iori
* @Date: 2022-02-14 11:29:20
**/
func (self *SignMgr) GetSignIns(keys ...string) *HttpSign {
	if len(keys) == 0 {
		return self.Instances["default"]
	} else {
		return self.Instances[keys[0]]
	}
}

/**
* @Description: 初始化签名管理器
* @receiver: self
* @param: cfgs
* @return: error
* @Author: Iori
* @Date: 2022-02-14 11:29:45
**/
func (self *SignMgr) InitAndRun(cfgs []Config) {
	logger.Infof("SignMgr   InsInit.. ")

	for _, conf := range cfgs {
		sign := BuildHttpSign(conf.SignName,
			conf.PrivateKey,
			int64(conf.ExpiredTime),
			conf.TimestampName,
			conf.RequestIDName,
			conf.VersionType,
		)
		self.SetSignIns(conf.IndexName, sign)
	}

	logger.Infof("SignMgr   InsInit... Done !")
}

/**
* @Description: tick
* @receiver: self
* @param: nowMs
* @Author: Iori
* @Date: 2022-02-14 11:39:54
**/
func (self *SignMgr) Tick(nowMs int64) {
	defer gfunc.CheckRecover()
	return
}
