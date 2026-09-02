// Package globals
// @Description: 全局服务管理器
package globals

import (
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
	"github.com/Iori372552686/GoOne/lib/web/rest_api"
)

var (
	SignMgr  = http_sign.NewSignMgr()
	RestMgr  = rest_api.NewRestApiMgr()
	RedisMgr = redis.NewRedisMgr()
)
