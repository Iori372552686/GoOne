package net_conf

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// nacos  client config struct
type NacosConf struct {
	IPAddr      string `json:"ip_addr"`
	Port        int    `json:"port"`
	NamespaceID string `json:"namespace_id"`
	GroupName   string `json:"group_name"`
	LogDir      string `json:"log_dir"`
	CacheDir    string `json:"cache_dir"`
	RotateTime  string `json:"rotate_time"`
	MaxAge      int    `json:"max_age"`
	LogLevel    string `json:"log_level"`
	UserName    string `json:"user_name"`
	Password    string `json:"password"`
}

func NewNacosConfigClient(conf NacosConf) config_client.IConfigClient {
	//server conf
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(conf.IPAddr, uint64(conf.Port)),
	}

	//client conf
	cc := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		NamespaceId:         conf.NamespaceID,
		LogDir:              conf.LogDir,
		CacheDir:            conf.CacheDir,
		LogLevel:            conf.LogLevel,
		Username:            conf.UserName,
		Password:            conf.Password,
	}

	// a more graceful way to create naming client
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		logger.Infof("NewConfigClient err | ", err.Error())
	}

	return client
}
