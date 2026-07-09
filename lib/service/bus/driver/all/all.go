// Package all registers every built-in bus driver (database/sql style).
//
//	import _ "github.com/Iori372552686/GoOne/lib/service/bus/driver/all"
//
// Services that want a smaller binary can instead blank-import only the
// drivers they actually use.
package all

import (
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/kafka"
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/nats"
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/nsq"
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	_ "github.com/Iori372552686/GoOne/lib/service/bus/driver/rocketmq"
)
