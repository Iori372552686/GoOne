package rds

import "github.com/Iori372552686/GoOne/lib/db/redis"

// RedisMgr 是 roomcentersvr 的 Redis 实例管理器。
// 用于房间数据快照持久化（任务 2.5），由 app.InitDeps 初始化。
var RedisMgr = redis.NewRedisMgr()
