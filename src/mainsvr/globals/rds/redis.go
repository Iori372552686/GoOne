package rds

import (
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/module/conf"
)

var RedisMgr = redis.NewRedisMgrWithConfigLoader(conf.Unmarshal)
