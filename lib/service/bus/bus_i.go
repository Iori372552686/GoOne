// Package bus 定义服务间消息总线的抽象（IBus）、显式 driver 注册表与地址解析。
//
// 具体 MQ 后端以驱动形式提供，位于 lib/service/bus/driver/<name>，每个驱动导出
// Driver() 描述符。服务在装配期通过 DriverRegistry 显式注册所需驱动，只链接用到的
// MQ SDK：
//
//	drivers := bus.NewDriverRegistry()
//	drivers.MustRegister(rabbitmq.Driver())
//
// 生产服务均经 DriverRegistry 装配（见各 src/<svc>/app.go），不再依赖包级 init
// 自注册或 driver/all 聚合包。
package bus

import (
	"context"
	"errors"
)

// cb  handler
type MsgHandler func(srcBusID uint32, data []byte) error

// 需保证协程并发安全
type IBus interface {
	// Start 同步完成首次连接与 consumer 注册（V4 P0-07）。
	//
	// 契约：返回 nil 表示 MQ 已连接、消费队列已声明、consumer 已创建，且消费/重连
	// goroutine 已启动。任一阶段失败按 ctx 返回 error，不启动后台 goroutine、不泄漏
	// 连接。Router 必须在 Start 成功后才注册服务发现，避免「被发现但 Bus 不可用」。
	// 已 Start 或已 Close 时返回 error。
	Start(ctx context.Context) error

	SelfBusId() uint32

	// Send 的成功语义（V4 P0-07）：返回 nil 至少表示消息已被当前连接成功 publish。
	// 不允许「记日志后返回 nil」的虚假成功。Close 后返回 ErrBusClosed。
	Send(dstBusId uint32, data1 []byte, data2 []byte) error

	// Close 取消并 join 全部后台 goroutine、关闭连接，幂等。
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
