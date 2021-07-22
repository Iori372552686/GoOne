package config

type DbInstance struct {
	InstanceId uint32
	Ip string
	Port int16
	User string
	Password string
	Schema string
	Description string
}

type Config struct {
	SelfBusId string
	ZKAddr string
	RabbitMQAddr string

	DbInstances []DbInstance
}

var SvrCfg Config

