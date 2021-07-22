package bus

type MsgHandler func(srcBusID uint32, data []byte)

// 需保证协程并发安全
type IBus interface {
	SelfBusId() uint32
	Send(dstBusId uint32, data1 []byte, data2 []byte) error

	// 默认规则：
	// 1. onRecvMsg由实现类的内部协程调用，且只会由一个协程调用。
	// 2. data的所有权，转交给onRecvMsg。
	// 如有例外，实现类需特殊说明。
	SetReceiver(onRecvMsg MsgHandler)
}