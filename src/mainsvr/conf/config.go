package config

type DbInstance struct {
	InstanceId uint32
	Ip string
	Port uint16
	Password string
	IsCluster bool
	Description string
}

type Config struct {
	SelfBusId string
	ZKAddr string
	RabbitMQAddr string

	Pprof bool
	GameDataDir string
	MongoDb_Ip string
  	MongoDb_Port string

	LogStashIP string
	LogStashPort int

	SensitiveWordsFile string
	DbInstances []DbInstance
}

var SvrCfg Config

