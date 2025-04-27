package bus

// implType : args
func CreateBus(implType string, selfBusId uint32, onRecvMsg MsgHandler, args ...interface{}) IBus {
	switch implType {
	case "nsq":
		return NewBusImplNsqMQ(selfBusId, onRecvMsg, args[0].(Config))

	case "rocketmq":
		//todo   -- need you!
		return nil

	default: //rbmq
		return NewBusImplRabbitMQ(selfBusId, onRecvMsg, args[0].(string))
	}
}
