package gconf

import (
	"flag"

	"github.com/Iori372552686/GoOne/lib/api/http_sign"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/api/rest_api"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
)

var SvrConfFile = flag.String("svr_conf", "../commconf/server_conf.yaml", "app config yaml file")

type BaseCfg struct {
	ZKAddr             string             `yaml:"ZKAddr"`             // zookeeper地址
	RabbitMQAddr       string             `yaml:"RabbitMQAddr"`       // rabbitmq地址
	GameDataDir        string             `yaml:"GameDataDir"`        // 游戏数据目录
	SensitiveWordsFile string             `yaml:"SensitiveWordsFile"` // 敏感词文件
	NacosConf          net_conf.NacosConf `yaml:"CenterConfAddr"`     // nacos配置
	OrmConf            []orm.Config       `yaml:"OrmInstances"`       // mysql配置
	HTTPSigns          []http_sign.Config `yaml:"HttpSign"`           // http签名配置
	RestApiConf        []rest_api.Config  `yaml:"RestApiConfig"`      // restapi配置
	DbInstances        []redis.Config     `yaml:"DbInstances"`        // redis配置
	Pprof              bool               `yaml:"Pprof"`              // 是否开启pprof
}

type ConnSvr struct {
	SelfBusId  string `yaml:"SelfBusId"`
	ListenPort int    `yaml:"ListenPort"`
	LogDir     string `yaml:"log_dir"`
	LogLevel   string `yaml:"log_level"`
}

type InfoSvr struct {
	SelfBusId string `yaml:"SelfBusId"`
	LogDir    string `yaml:"log_dir"`
	LogLevel  string `yaml:"log_level"`
}

type MainSvr struct {
	SelfBusId string `yaml:"SelfBusId"`
	LogDir    string `yaml:"log_dir"`
	LogLevel  string `yaml:"log_level"`
}

type MySqlSvr struct {
	SelfBusId string `yaml:"SelfBusId"`
	LogDir    string `yaml:"log_dir"`
	LogLevel  string `yaml:"log_level"`
}

type RoomCenterSvr struct {
	SelfBusId string `yaml:"SelfBusId"`
	LogDir    string `yaml:"log_dir"`
	LogLevel  string `yaml:"log_level"`
}

type TexasSvr struct {
	SelfBusId string `yaml:"SelfBusId"`
	LogDir    string `yaml:"log_dir"`
	LogLevel  string `yaml:"log_level"`
}

// connsvr配置
type ConnConfig struct {
	BaseCfg `yaml:"basecfg"`
	ConnSvr `yaml:"connsvr"`
}

var ConnSvrCfg ConnConfig

// infosvr配置
type InfoConfig struct {
	BaseCfg `yaml:"basecfg"`
	InfoSvr `yaml:"infosvr"`
}

var InfoSvrCfg InfoConfig

// mainsvr配置
type MainSvrConfig struct {
	BaseCfg `yaml:"basecfg"`
	MainSvr `yaml:"mainsvr"`
}

var MainSvrCfg MainSvrConfig

// mysqlsvr配置
type MySqlSvrConfig struct {
	BaseCfg  `yaml:"basecfg"`
	MySqlSvr `yaml:"mysqlsvr"`
}

var MySqlSvrCfg MySqlSvrConfig

// roomcentersvr配置
type RoomCenterConfig struct {
	BaseCfg       `yaml:"basecfg"`
	RoomCenterSvr `yaml:"roomcentersvr"`
}

var RoomCenterSvrCfg RoomCenterConfig

// texassvr配置
type TexasConfig struct {
	BaseCfg  `yaml:"basecfg"`
	TexasSvr `yaml:"texassvr"`
}

var TexasSvrCfg TexasConfig
