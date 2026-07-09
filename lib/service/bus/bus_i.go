// Package bus 定义服务间消息总线的抽象（IBus）、驱动注册工厂与地址解析。
//
// 具体 MQ 后端以驱动形式提供（database/sql 风格），位于
// lib/service/bus/driver/<name>，通过 blank import 按需编入：
//
//	import _ "github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
//	// 或一次引入全部：
//	import _ "github.com/Iori372552686/GoOne/lib/service/bus/driver/all"
//
// bootstrap/busapp 默认引入 driver/all；需要裁剪二进制体积的服务可自行
// 装配并只引入所需驱动。
package bus

import (
	"errors"
)

// cb  handler
type MsgHandler func(srcBusID uint32, data []byte) error

// 需保证协程并发安全
type IBus interface {
	SelfBusId() uint32
	Send(dstBusId uint32, data1 []byte, data2 []byte) error
	Close() error

	// Healthy 返回当前与 MQ 的连接是否就绪（用于 readyz 健康检查）。
	Healthy() bool

	// 默认规则：
	// 1. onRecvMsg由实现类的内部协程调用，且只会由一个协程调用。
	// 2. data的所有权，转交给onRecvMsg。
	// 如有例外，实现类需特殊说明。
	SetReceiver(onRecvMsg MsgHandler)
}

// ErrBusClosed is returned by Send after Close.
var ErrBusClosed = errors.New("bus is closed")
