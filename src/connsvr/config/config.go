package config

type Config struct {
	SelfBusId string
	ZKAddr string
	RabbitMQAddr string
	LoginSdkAddr string
	ListenPort int
}

var SvrCfg Config
